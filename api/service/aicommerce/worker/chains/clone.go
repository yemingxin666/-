package chains

import (
	"context"
	"fmt"
	logger2 "geekai/logger"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// cloneStyleImageTimeout 克隆设计单张风格参考图的处理超时预算。
// 业务规则：按风格参考图张数计算超时，每张 90s；当前每次调用处理 1 张风格图。
const cloneStyleImageTimeout = 90 * time.Second

var cloneLogger = logger2.GetLogger()

// RunClone 克隆设计链：
//   - reference_assets：产品参考图（描述目标产品身份）
//   - clone_assets：风格参考图，每张作为 ImageToImage source，输出 1 张克隆结果
//
// 失败策略（fail-soft）：
//  1. 全部成功：return nil，dispatcher 标记 succeeded
//  2. 部分成功：内部退还失败部分，更新 task.CreditCost，return nil
//  3. 全部失败：return firstErr，dispatcher 全额退款并标记 failed
func RunClone(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	productName, _ := input["product_name"].(string)
	styleDesc, _ := input["style_desc"].(string)
	sellingPoints, _ := input["selling_points"].(string)
	referenceAssetNos, _ := extractStringSlice(input, "reference_assets")
	cloneAssetNos, _ := extractStringSlice(input, "clone_assets")

	if len(referenceAssetNos) == 0 {
		return fmt.Errorf("clone: no product reference image provided")
	}
	if len(cloneAssetNos) == 0 {
		return fmt.Errorf("clone: no clone style image provided")
	}

	total := len(cloneAssetNos)
	updateProgress(db, task, 5)

	// 校验产品图可访问性（不直接用于生图，仅校验存在）
	productURLs := resolveAssetURLs(db, task.UserId, referenceAssetNos, cfg, uploader)
	if len(productURLs) == 0 {
		return fmt.Errorf("clone: no accessible product reference image")
	}

	// 先全部创建占位 asset，前端轮询可立即看到 N 张"进行中"
	placeholderIDs := make([]uint, total)
	for i := range cloneAssetNos {
		placeholderIDs[i] = createPhaseAsset(db, task, cloneImageType(i), PhaseRendering)
	}

	var (
		firstErr  error
		succeeded int
	)

	for i, cloneAssetNo := range cloneAssetNos {
		imageType := cloneImageType(i)
		placeholderID := placeholderIDs[i]

		if err := processOneClone(
			ctx, db, imgClient, uploader, cfg, task,
			productName, sellingPoints, styleDesc,
			referenceAssetNos, cloneAssetNo,
			imageType, placeholderID,
		); err != nil {
			saveTypeError(db, task, imageType, err.Error(), placeholderID)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
		}

		progress := 10 + int(float64(i+1)/float64(total)*85)
		if progress > 95 {
			progress = 95
		}
		updateProgress(db, task, progress)
	}

	if succeeded == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("clone: no image generated successfully")
	}

	// 部分成功：内部退款，避免 dispatcher 走全退路径
	if succeeded < total {
		if err := refundFailedCloneCredits(db, task, total, succeeded, aicommerce.CloneCreditPerImage); err != nil {
			// 退款失败不向上抛，防止整任务被标记 failed 触发双重退款
			// 但必须告警让运维介入手动补偿，避免用户被多扣算力
			cloneLogger.Errorf("[clone] task=%d user=%d partial refund failed: total=%d succeeded=%d unit=%d err=%v",
				task.Id, task.UserId, total, succeeded, aicommerce.CloneCreditPerImage, err)
			_ = db.Model(task).Update("error_message", fmt.Sprintf("clone partial refund failed: %v", err)).Error
		}
	}

	return nil
}

// processOneClone 处理单张风格参考图：风格图作 source，按 phase 推进
func processOneClone(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
	productName, sellingPoints, styleDesc string,
	referenceAssetNos []string,
	cloneAssetNo string,
	imageType string,
	placeholderID uint,
) error {
	styleURLs := resolveAssetURLs(db, task.UserId, []string{cloneAssetNo}, cfg, uploader)
	if len(styleURLs) == 0 {
		return fmt.Errorf("clone style asset %s not found or inaccessible", cloneAssetNo)
	}

	// 解析产品参考图 URL（多图 provider 会一并送入；单图 provider 静默忽略）
	productURLs := resolveAssetURLs(db, task.UserId, referenceAssetNos, cfg, uploader)
	if len(productURLs) == 0 {
		return fmt.Errorf("clone: product reference URLs unavailable")
	}

	// 占位 asset 创建时已是 PhaseRendering，此处直接渲染 prompt 后进入 generating
	// Prompt 中按"风格图在前、产品图在后"的顺序标注图片角色，与 content blocks 顺序严格对齐
	stylePrompt := buildClonePrompt(db, task, productName, sellingPoints, styleDesc, len(productURLs))

	updatePhaseAsset(db, placeholderID, PhaseGenerating)

	// 按风格参考图张数分配超时：每张 90s，processOneClone 单次仅处理 1 张
	callCtx, cancel := context.WithTimeout(ctx, cloneStyleImageTimeout)
	defer cancel()

	result, err := imgClient.ImageToImage(callCtx, provider.ImageToImageReq{
		Model:          task.Model,
		Prompt:         stylePrompt,
		ImageURL:       styleURLs[0], // Image 1：风格参考图
		ExtraImageURLs: productURLs,  // Image 2..N+1：产品参考图
		ImageSize:      provider.RatioToSize(task.Ratio),
		Strength:       0.7,
	})
	if err != nil {
		return fmt.Errorf("clone image %s: %w", cloneAssetNo, err)
	}
	if len(result.Images) == 0 {
		return fmt.Errorf("clone image %s: no images returned", cloneAssetNo)
	}

	updatePhaseAsset(db, placeholderID, PhaseUploading)

	img := result.Images[0]
	ossKey, err := ossUploadURL(uploader, img.URL)
	if err != nil {
		return fmt.Errorf("upload clone image %s: %w", cloneAssetNo, err)
	}

	return finalizePhaseAsset(db, placeholderID, ossKey, cfg.OSSBucket, "image/png", img.Width, img.Height,
		model.JSONMap{
			"image_type":            imageType,
			"source_style_asset_no": cloneAssetNo,
			"product_reference_nos": referenceAssetNos,
			"ratio":                 task.Ratio,
			"generation_strategy":   "style_plus_product_multi_image",
			"output_credit_charged": aicommerce.CloneCreditPerImage,
			"positive_prompt":       stylePrompt,
		})
}

// buildClonePrompt 构建克隆设计 Prompt：
//   - Image 1 是风格参考图（仅借用其视觉风格 / 构图 / 灯光 / 色调）
//   - Image 2..N+1 是产品参考图（产品主体身份必须以这些图为准）
//
// productImageCount 用于在 prompt 中显式声明产品图数量，使模型能将 content blocks
// 中第 2..N+1 张图绑定为产品身份来源。
func buildClonePrompt(
	db *gorm.DB,
	task *model.AiImageTask,
	productName, sellingPoints, styleDesc string,
	productImageCount int,
) string {
	parts := []string{
		"E-commerce product image generation with multiple reference images.",
		fmt.Sprintf("You are given %d reference image(s) in total. Image 1 is the STYLE reference. Image 2 to Image %d are PRODUCT references.", productImageCount+1, productImageCount+1),
		"STYLE reference (Image 1): use ONLY for visual style, layout, lighting, composition, color palette, background mood, camera angle and photography feel. NEVER copy its product identity, logo, watermark, text, brand, packaging or protected design.",
		fmt.Sprintf("PRODUCT references (Image 2..Image %d): the generated product MUST be the exact same product shown in these images — preserve product identity, category, silhouette, material, color, texture, structural details and any printed graphics/logos that belong to this product.", productImageCount+1),
		"Compose the final image by placing the PRODUCT (from Image 2..N) into the STYLE/scene of Image 1. Do not invent a different product.",
	}
	if name := strings.TrimSpace(productName); name != "" {
		parts = append(parts, fmt.Sprintf("Target product name: %s.", name))
	}
	if sp := strings.TrimSpace(sellingPoints); sp != "" {
		parts = append(parts, fmt.Sprintf("Target selling points: %s.", sp))
	}
	parts = append(parts,
		fmt.Sprintf("Platform requirements: %s.", prompt.PlatformRules(db, task.Platform)),
		fmt.Sprintf("Output language: %s.", strings.TrimSpace(task.Language)),
		fmt.Sprintf("Aspect ratio: %s.", strings.TrimSpace(task.Ratio)),
		"High quality, professional commercial photography, clean product presentation, realistic details.",
	)
	if s := strings.TrimSpace(styleDesc); s != "" {
		parts = append(parts, fmt.Sprintf("Additional desired style: %s.", s))
	}
	return strings.Join(parts, "\n")
}

// cloneImageType 合成与前端占位 image_type 对齐的标识：clone_0、clone_1...
func cloneImageType(index int) string {
	return fmt.Sprintf("clone_%d", index)
}

// refundFailedCloneCredits 部分失败退款：事务内增加用户算力 + 修正 task.credit_cost
func refundFailedCloneCredits(db *gorm.DB, task *model.AiImageTask, total, succeeded, unitCost int) error {
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
			return fmt.Errorf("refund clone credits: %w", err)
		}
		if err := tx.Model(task).Update("credit_cost", finalCost).Error; err != nil {
			return fmt.Errorf("update clone credit_cost: %w", err)
		}
		task.CreditCost = finalCost
		return nil
	})
}
