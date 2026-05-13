package chains

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"time"

	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"

	"gorm.io/gorm"
)

// RatioConvertCreditPerImage 比例转换每张图的算力单价（outpaint 模式）
// crop 模式使用 RatioConvertCropCredit
const (
	RatioConvertOutpaintCredit = 10
	RatioConvertCropCredit     = 3
)

// ratioConvertPerImageTimeout outpaint 模式每张图的超时预算
const ratioConvertPerImageTimeout = 90 * time.Second

// RunRatioConvert 比例转换链：支持多图批量处理，裁剪 or Outpaint
//
// 失败策略（fail-soft，与 clone 一致）：
//  1. 全部成功：return nil，dispatcher 标记 succeeded
//  2. 部分成功：内部退还失败部分算力，return nil
//  3. 全部失败：return firstErr，dispatcher 全额退款并标记 failed
func RunRatioConvert(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	assetNos, _ := extractStringSlice(input, "reference_assets")

	// 兼容前端：mode 字段存储在 style_desc 中（前端 RatioConvertPage 传 style_desc: mode.value）
	mode, _ := input["style_desc"].(string)
	if mode == "" {
		mode, _ = input["mode"].(string) // 向后兼容
	}
	if mode == "" {
		mode = "outpaint" // 默认 outpaint
	}

	if len(assetNos) == 0 {
		return fmt.Errorf("ratio_convert: no source image provided")
	}

	total := len(assetNos)
	updateProgress(db, task, 5)

	// 批量查询所有参考图 asset
	var srcAssets []model.AiImageAsset
	if err := db.Where("asset_no IN ? AND user_id = ? AND deleted_at IS NULL", assetNos, task.UserId).
		Find(&srcAssets).Error; err != nil {
		return fmt.Errorf("ratio_convert: query assets failed: %w", err)
	}
	assetMap := make(map[string]model.AiImageAsset, len(srcAssets))
	for _, a := range srcAssets {
		assetMap[a.AssetNo] = a
	}

	// 为每张图创建占位 asset（前端可立即看到 N 张"进行中"）
	placeholderIDs := make([]uint, total)
	for i := range assetNos {
		placeholderIDs[i] = createPhaseAsset(db, task, ratioConvertImageType(i), PhaseRendering)
	}

	var (
		firstErr      error
		succeeded     int
		actualCropCnt int // auto 模式下实际走 crop 的数量，用于退差价
	)

	for i, assetNo := range assetNos {
		imageType := ratioConvertImageType(i)
		placeholderID := placeholderIDs[i]

		srcAsset, ok := assetMap[assetNo]
		if !ok {
			errMsg := fmt.Sprintf("asset %s not found", assetNo)
			saveTypeError(db, task, imageType, errMsg, placeholderID)
			if firstErr == nil {
				firstErr = fmt.Errorf(errMsg)
			}
			continue
		}

		actualMode, err := processOneRatioConvert(
			ctx, db, imgClient, uploader, cfg, task,
			srcAsset, mode, imageType, placeholderID,
		)
		if err != nil {
			saveTypeError(db, task, imageType, err.Error(), placeholderID)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
			if actualMode == "crop" {
				actualCropCnt++
			}
		}

		// 线性进度：5% ~ 95%
		progress := 5 + int(float64(i+1)/float64(total)*90)
		if progress > 95 {
			progress = 95
		}
		updateProgress(db, task, progress)
	}

	if succeeded == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("ratio_convert: no image processed successfully")
	}

	// 部分成功：退还失败部分的算力
	if succeeded < total {
		unitCost := ratioConvertUnitCost(mode)
		if err := refundFailedRatioConvertCredits(db, task, total, succeeded, unitCost); err != nil {
			_ = db.Model(task).Update("error_message",
				fmt.Sprintf("ratio_convert partial refund failed: %v", err)).Error
		}
	}

	// auto 模式差价退款：预收 outpaint 价，实际走 crop 时退还差价（10-3=7）
	if mode == "auto" && actualCropCnt > 0 {
		diff := (RatioConvertOutpaintCredit - RatioConvertCropCredit) * actualCropCnt
		if err := refundAutoModeDiff(db, task, diff); err != nil {
			_ = db.Model(task).Update("error_message",
				fmt.Sprintf("ratio_convert auto diff refund failed: %v", err)).Error
		}
	}

	return nil
}

// processOneRatioConvert 处理单张图片的比例转换，返回实际使用的模式（"crop"/"outpaint"）
func processOneRatioConvert(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
	srcAsset model.AiImageAsset,
	mode string,
	imageType string,
	placeholderID uint,
) (actualMode string, err error) {
	updatePhaseAsset(db, placeholderID, PhaseGenerating)

	srcURL := resolveAssetURLs(db, task.UserId, []string{srcAsset.AssetNo}, cfg, uploader)
	if len(srcURL) == 0 {
		return "", fmt.Errorf("source asset %s URL unavailable", srcAsset.AssetNo)
	}

	var (
		ossKey   string
		width    int
		height   int
		mimeType string
	)

	// 判断是否使用裁剪模式
	useCrop := mode == "crop" || (mode == "auto" && canCropSmart(srcAsset, task.Ratio))

	if useCrop {
		// 智能裁剪：下载原图 → 计算最优裁剪区域 → 裁剪 → 上传
		ossKey, width, height, mimeType, err = cropAndUpload(ctx, uploader, srcURL[0], srcAsset, task.Ratio)
		if err != nil {
			return "", fmt.Errorf("crop %s: %w", srcAsset.AssetNo, err)
		}
	} else {
		// Outpaint 扩图：先本地 padding，再交给 AI 仅融合边缘
		// 目的：原图区域作为强参考（含文字/logo 等），AI 主要工作是把 padding 边缘自然融入
		callCtx, cancel := context.WithTimeout(ctx, ratioConvertPerImageTimeout)
		defer cancel()

		// 1. 下载原图 → padding 到目标比例 → 上传 OSS 作为参考输入
		paddedKey, paddedURL, perr := buildPaddedReference(callCtx, uploader, srcURL[0], task.Ratio, cfg)
		if perr != nil {
			return "", fmt.Errorf("pad reference %s: %w", srcAsset.AssetNo, perr)
		}
		_ = paddedKey // 中间产物，仅用于本次 AI 调用，不入库

		genReq := provider.ImageToImageReq{
			Model:     task.Model,
			Prompt:    buildOutpaintPrompt(task.Ratio),
			ImageURL:  paddedURL,
			ImageSize: provider.RatioToSize(task.Ratio),
			Strength:  0.2, // 低强度：尽量保留原图内容，仅让 AI 平滑融合 padding 边缘
		}
		var result *provider.GenerateResult
		result, err = imgClient.ImageToImage(callCtx, genReq)
		if err != nil {
			return "", fmt.Errorf("outpaint %s: %w", srcAsset.AssetNo, err)
		}
		if len(result.Images) == 0 {
			return "", fmt.Errorf("outpaint %s: no images returned", srcAsset.AssetNo)
		}

		img := result.Images[0]
		ossKey, err = ossUploadURL(uploader, img.URL)
		if err != nil {
			return "", fmt.Errorf("upload outpaint result %s: %w", srcAsset.AssetNo, err)
		}
		width = img.Width
		height = img.Height
		mimeType = "image/png"
	}

	updatePhaseAsset(db, placeholderID, PhaseUploading)

	actualMode = modeLabel(useCrop)
	if err = finalizePhaseAsset(db, placeholderID, ossKey, cfg.OSSBucket, mimeType, width, height,
		model.JSONMap{
			"image_type":       imageType,
			"source_asset_no":  srcAsset.AssetNo,
			"target_ratio":     task.Ratio,
			"convert_mode":     mode,
			"actual_mode_used": actualMode,
		}); err != nil {
		return "", err
	}
	return actualMode, nil
}

// cropAndUpload 下载原图，智能裁剪到目标比例，上传到 OSS
// 裁剪策略：中心加权裁剪 — 保留图片中心区域，按目标比例裁切最大矩形
func cropAndUpload(
	ctx context.Context,
	uploader oss.Uploader,
	srcURL string,
	srcAsset model.AiImageAsset,
	targetRatio string,
) (ossKey string, outW, outH int, mimeType string, err error) {
	// 1. 下载原图
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	// 限制下载大小 20MB
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("read image: %w", err)
	}

	// 2. 解码图片
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// 3. 计算裁剪区域
	targetW, targetH := parseRatio(targetRatio)
	if targetW <= 0 || targetH <= 0 {
		return "", 0, 0, "", fmt.Errorf("invalid target ratio: %s", targetRatio)
	}
	cropRect := computeCropRect(srcW, srcH, targetW, targetH)

	// 4. 执行裁剪
	cropped := cropImage(img, cropRect)
	outW = cropRect.Dx()
	outH = cropRect.Dy()

	// 5. 编码：PNG 保留透明通道，其他格式转 JPEG
	var buf bytes.Buffer
	if format == "png" {
		err = png.Encode(&buf, cropped)
		mimeType = "image/png"
	} else {
		err = jpeg.Encode(&buf, cropped, &jpeg.Options{Quality: 92})
		mimeType = "image/jpeg"
	}
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("encode cropped image: %w", err)
	}

	// 6. 上传到 OSS
	ext := ".jpg"
	if format == "png" {
		ext = ".png"
	}
	ossKey, err = uploader.PutBytes(buf.Bytes(), ext)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("upload cropped image: %w", err)
	}

	return ossKey, outW, outH, mimeType, nil
}

// computeCropRect 计算中心加权裁剪区域
// 策略：在源图中找到目标比例的最大内接矩形，居中裁剪
func computeCropRect(srcW, srcH, ratioW, ratioH int) image.Rectangle {
	targetAspect := float64(ratioW) / float64(ratioH)
	srcAspect := float64(srcW) / float64(srcH)

	var cropW, cropH int
	if srcAspect > targetAspect {
		// 源图更宽：以高度为基准，裁宽度
		cropH = srcH
		cropW = int(math.Round(float64(srcH) * targetAspect))
	} else {
		// 源图更高：以宽度为基准，裁高度
		cropW = srcW
		cropH = int(math.Round(float64(srcW) / targetAspect))
	}

	// 确保不超出边界
	if cropW > srcW {
		cropW = srcW
	}
	if cropH > srcH {
		cropH = srcH
	}

	// 居中
	x0 := (srcW - cropW) / 2
	y0 := (srcH - cropH) / 2

	return image.Rect(x0, y0, x0+cropW, y0+cropH)
}

// cropImage 从原图中裁剪指定区域
func cropImage(img image.Image, rect image.Rectangle) image.Image {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(rect)
	}
	// fallback：逐像素复制（几乎不会走到这里，标准库 image 类型都实现了 SubImage）
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dst.Set(x-rect.Min.X, y-rect.Min.Y, img.At(x, y))
		}
	}
	return dst
}

// canCropSmart 智能判断是否可以裁剪（不丢失过多内容）
// 规则：裁剪后保留面积 >= 原图面积的 60% 时才允许裁剪
func canCropSmart(asset model.AiImageAsset, targetRatio string) bool {
	if asset.Width == 0 || asset.Height == 0 {
		return false
	}

	ratioW, ratioH := parseRatio(targetRatio)
	if ratioW <= 0 || ratioH <= 0 {
		return false
	}
	rect := computeCropRect(asset.Width, asset.Height, ratioW, ratioH)
	cropArea := float64(rect.Dx()) * float64(rect.Dy())
	srcArea := float64(asset.Width) * float64(asset.Height)

	// 保留面积 >= 60% 才允许裁剪
	return cropArea/srcArea >= 0.60
}

// buildOutpaintPrompt 构建 outpaint 扩图的 prompt
// 注意：输入图已经在后端 padding 到目标比例，中心是完整原图（含文字/logo/产品），
// 四周是用边缘像素延展的 padding 区域。AI 的任务是仅自然融合 padding 边缘，
// 严禁改动中心原图区域（文字、logo、产品必须 1:1 保留）。
func buildOutpaintPrompt(ratio string) string {
	return fmt.Sprintf(
		"The input image is already in %s aspect ratio. "+
			"The center area contains the COMPLETE original image including all text, logos, watermarks, products and graphics — preserve it EXACTLY pixel-by-pixel, do NOT redraw, modify, translate, or remove any text or graphic in the center. "+
			"Only the outer border padding area needs natural blending: extend the existing background pattern smoothly so the edges look seamless. "+
			"Keep lighting, color tone, and texture consistent with the original. "+
			"Do NOT add new objects, text, or watermarks anywhere.",
		ratio,
	)
}

// buildPaddedReference 下载原图，按目标比例 padding 后上传，返回 OSS key 与可访问 URL。
// padding 策略：原图居中，四周用边缘像素颜色作为底色填充（避免纯白突兀，便于 AI 融合）。
// 失败时调用方应回退或直接报错。
func buildPaddedReference(
	ctx context.Context,
	uploader oss.Uploader,
	srcURL string,
	targetRatio string,
	cfg aicommerce.Config,
) (ossKey, accessURL string, err error) {
	// 1. 下载原图
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", "", fmt.Errorf("read image: %w", err)
	}

	// 2. 解码
	srcImg, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", "", fmt.Errorf("decode image: %w", err)
	}
	srcBounds := srcImg.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return "", "", fmt.Errorf("invalid source size: %dx%d", srcW, srcH)
	}

	// 3. 计算 padded 画布尺寸：以原图为基准，按目标比例向短边方向扩展
	rw, rh := parseRatio(targetRatio)
	if rw <= 0 || rh <= 0 {
		return "", "", fmt.Errorf("invalid target ratio: %s", targetRatio)
	}
	canvasW, canvasH := computePaddedCanvas(srcW, srcH, rw, rh)
	if canvasW == srcW && canvasH == srcH {
		// 已是目标比例，无需 padding，直接复用原图 URL
		// 上层用 srcURL 也能跑，但为保持调用方一致这里仍返回原 URL
		return "", srcURL, nil
	}

	// 4. 绘制 padded 画布：用原图四边像素的平均色作为底色
	bgColor := edgeAverageColor(srcImg)
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// 5. 原图居中贴入
	offX := (canvasW - srcW) / 2
	offY := (canvasH - srcH) / 2
	draw.Draw(canvas,
		image.Rect(offX, offY, offX+srcW, offY+srcH),
		srcImg, srcBounds.Min, draw.Src)

	// 6. 编码 PNG 并上传 OSS
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return "", "", fmt.Errorf("encode padded png: %w", err)
	}
	ossKey, err = uploader.PutBytes(buf.Bytes(), ".png")
	if err != nil {
		return "", "", fmt.Errorf("upload padded image: %w", err)
	}

	// 7. 构造可访问 URL（公网 bucket 直接拼接；私有 bucket 需签名，此处沿用 signedURL）
	accessURL = signedURL(ossKey, cfg)
	return ossKey, accessURL, nil
}

// computePaddedCanvas 计算 padding 后的画布尺寸：
// 保持原图完整，按目标比例向短边方向扩展画布。
func computePaddedCanvas(srcW, srcH, ratioW, ratioH int) (canvasW, canvasH int) {
	targetAspect := float64(ratioW) / float64(ratioH)
	srcAspect := float64(srcW) / float64(srcH)

	if math.Abs(srcAspect-targetAspect) < 1e-6 {
		// 比例已匹配
		return srcW, srcH
	}
	if srcAspect < targetAspect {
		// 源图偏窄（更高）：加宽
		canvasH = srcH
		canvasW = int(math.Round(float64(srcH) * targetAspect))
		if canvasW < srcW {
			canvasW = srcW
		}
		return canvasW, canvasH
	}
	// 源图偏宽：加高
	canvasW = srcW
	canvasH = int(math.Round(float64(srcW) / targetAspect))
	if canvasH < srcH {
		canvasH = srcH
	}
	return canvasW, canvasH
}

// edgeAverageColor 计算原图四边像素的平均色，作为 padding 底色（比纯白更易融合）。
func edgeAverageColor(img image.Image) color.RGBA {
	b := img.Bounds()
	var rSum, gSum, bSum, count uint64
	addPixel := func(x, y int) {
		r, g, bl, _ := img.At(x, y).RGBA()
		rSum += uint64(r >> 8)
		gSum += uint64(g >> 8)
		bSum += uint64(bl >> 8)
		count++
	}
	// 采样四边，每边间隔取一定步长，避免遍历过密
	step := b.Dx() / 64
	if step < 1 {
		step = 1
	}
	for x := b.Min.X; x < b.Max.X; x += step {
		addPixel(x, b.Min.Y)
		addPixel(x, b.Max.Y-1)
	}
	step = b.Dy() / 64
	if step < 1 {
		step = 1
	}
	for y := b.Min.Y; y < b.Max.Y; y += step {
		addPixel(b.Min.X, y)
		addPixel(b.Max.X-1, y)
	}
	if count == 0 {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.RGBA{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
		A: 255,
	}
}

// ratioConvertImageType 生成与前端占位 image_type 对齐的标识
func ratioConvertImageType(index int) string {
	return fmt.Sprintf("ratio_convert_%d", index)
}

// ratioConvertUnitCost 根据模式返回单张图的算力消耗
func ratioConvertUnitCost(mode string) int {
	if mode == "crop" {
		return RatioConvertCropCredit
	}
	return RatioConvertOutpaintCredit
}

func modeLabel(usedCrop bool) string {
	if usedCrop {
		return "crop"
	}
	return "outpaint"
}

// refundFailedRatioConvertCredits 部分失败退款
func refundFailedRatioConvertCredits(db *gorm.DB, task *model.AiImageTask, total, succeeded, unitCost int) error {
	if task == nil || total <= 0 || succeeded < 0 || succeeded >= total || unitCost <= 0 {
		return nil
	}
	failed := total - succeeded
	refund := failed * unitCost
	finalCost := succeeded * unitCost
	if refund <= 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", refund)).Error; err != nil {
			return fmt.Errorf("refund ratio_convert credits: %w", err)
		}
		if err := tx.Model(task).Update("credit_cost", finalCost).Error; err != nil {
			return fmt.Errorf("update ratio_convert credit_cost: %w", err)
		}
		task.CreditCost = finalCost
		return nil
	})
}

// refundAutoModeDiff auto 模式差价退款：预收 outpaint 价，实际走 crop 时退（10-3）差价
func refundAutoModeDiff(db *gorm.DB, task *model.AiImageTask, diff int) error {
	if diff <= 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", diff)).Error; err != nil {
			return fmt.Errorf("refund auto diff: %w", err)
		}
		newCost := task.CreditCost - diff
		if newCost < 0 {
			newCost = 0
		}
		if err := tx.Model(task).Update("credit_cost", newCost).Error; err != nil {
			return fmt.Errorf("update credit_cost after auto diff: %w", err)
		}
		task.CreditCost = newCost
		return nil
	})
}
