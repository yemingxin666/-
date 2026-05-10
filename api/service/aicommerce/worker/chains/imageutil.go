package chains

// 图像合成工具函数。
// 当前仅服务于 white_bg chain 的"抠图 → 白底画布合成"步骤，
// 后续 ratio_convert 的 crop/outpaint 场景也可复用。

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // 允许 image.Decode 识别 JPEG 输入
	"image/png"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
)

// 画布策略参数
const (
	// whiteBgMargin 产品与画布四边的相对留白比例，避免主体贴边
	whiteBgMargin = 0.08
	// whiteBgBaseLong 无法从源图推断尺寸时，画布长边的最小兜底值
	whiteBgBaseLong = 1024
)

// CompositeWhiteBg 将透明背景的产品图居中合成到按 ratio 生成的白色画布上。
//
// 输入：src 通常是阿里云视觉「背景移除」返回的透明 PNG；
//
//	ratio 形如 "1:1"/"3:4"/"16:9"，非法或空时按源图尺寸出图（仅换白底）。
//
// 输出：PNG 字节流 + 画布宽高。失败时返回解析/编码错误。
//
// 为什么居中 + 等比缩放：电商白底主图约定主体完整可见、四周留白，
// 直接按画布铺满会拉伸变形，这里保持产品比例、按较短边 fit。
func CompositeWhiteBg(src io.Reader, ratio string) ([]byte, int, int, error) {
	srcImg, _, err := image.Decode(src)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode source image: %w", err)
	}
	srcBounds := srcImg.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid source image size: %dx%d", srcW, srcH)
	}

	canvasW, canvasH := calcCanvasDims(srcW, srcH, ratio)

	// 产品可用区域：画布去除四边 margin 后的内接矩形
	maxW := int(math.Round(float64(canvasW) * (1 - 2*whiteBgMargin)))
	maxH := int(math.Round(float64(canvasH) * (1 - 2*whiteBgMargin)))
	if maxW <= 0 || maxH <= 0 {
		maxW, maxH = canvasW, canvasH
	}

	// 按较短边 fit：只允许缩小不放大，避免小图被硬拉糊
	scale := math.Min(float64(maxW)/float64(srcW), float64(maxH)/float64(srcH))
	if scale > 1 {
		scale = 1
	}
	newW := int(math.Round(float64(srcW) * scale))
	newH := int(math.Round(float64(srcH) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	resized := resize.Resize(uint(newW), uint(newH), srcImg, resize.Lanczos3)

	// 画纯白底
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	// 居中贴入透明产品图；draw.Over 保证 alpha 通道在白底上正确混合
	offX := (canvasW - newW) / 2
	offY := (canvasH - newH) / 2
	draw.Draw(canvas,
		image.Rect(offX, offY, offX+newW, offY+newH),
		resized, image.Point{}, draw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, 0, 0, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), canvasW, canvasH, nil
}

// calcCanvasDims 根据比例和源图大小确定目标画布尺寸。
// 规则：取 max(srcLongSide, whiteBgBaseLong) 作为长边基准，按 ratio 推算短边。
// 这样小图也至少上到 1024，大图保留原分辨率，不过度放大也不过度压缩。
func calcCanvasDims(srcW, srcH int, ratio string) (int, int) {
	rw, rh := parseRatio(ratio)
	if rw <= 0 || rh <= 0 {
		// 比例非法：维持源图尺寸，至少只做替换白底，不变形
		return srcW, srcH
	}
	longBase := srcW
	if srcH > longBase {
		longBase = srcH
	}
	if longBase < whiteBgBaseLong {
		longBase = whiteBgBaseLong
	}
	if rw >= rh {
		// 横向或正方形：宽=长边
		return longBase, int(math.Round(float64(longBase) * float64(rh) / float64(rw)))
	}
	// 竖向：高=长边
	return int(math.Round(float64(longBase) * float64(rw) / float64(rh))), longBase
}

func parseRatio(ratio string) (int, int) {
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return 0, 0
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil {
		return 0, 0
	}
	return w, h
}
