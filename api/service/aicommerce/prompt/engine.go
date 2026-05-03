package prompt

import (
	"bytes"
	"fmt"
	"html"
	"strings"
	"text/template"
)

// Vars 模板变量集合
type Vars struct {
	// 基础字段
	ProductName         string
	SellingPoints       string
	ImageTypeDesc       string
	Platform            string
	PlatformRules       string
	Language            string
	Ratio               string
	StyleDesc           string
	ReferenceImageCount int

	// AI 视觉分析扩展字段（来自 CopywriteAnalysis）
	ProductDescForPrompt string // 英文 Prompt 描述，含颜色/款型/材质等视觉细节
	ProductType          string // 商品品类
	GarmentPosition      string // top | bottom | full-body | non-apparel
	Color                string // 精确英文色值描述
	Material             string // 主要材质
	Style                string // 版型描述
	PrintDesign          string // 印花/设计描述
	PrintDesignLock      string // 精确约束短语，防止印花变形
	TargetAudience       string // 目标人群
	ProductStyle         string // 商品风格
	ProductNameZh        string // 中文商品名简短版

	// 结构化卖点（最多5条）
	SP0Zh     string
	SP0En     string
	SP0ZhDesc string
	SP0EnDesc string
	SP1Zh     string
	SP1En     string
	SP1ZhDesc string
	SP1EnDesc string
	SP2Zh     string
	SP2En     string
	SP2ZhDesc string
	SP2EnDesc string
	SP3Zh     string
	SP3En     string
	SP3ZhDesc string
	SP3EnDesc string
	SP4Zh     string
	SP4En     string
	SP4ZhDesc string
	SP4EnDesc string

	// 目标使用场景（最多3条，中英双语，用于场景代入图/多场景拼图）
	Scene0Zh string
	Scene0En string
	Scene1Zh string
	Scene1En string
	Scene2Zh string
	Scene2En string
}

// RenderResult 渲染结果
type RenderResult struct {
	SystemPrompt   string
	PositivePrompt string
	NegativePrompt string
}

// Render 渲染模板为最终 Prompt
func Render(tmplText, negativeTmpl string, vars Vars) (*RenderResult, error) {
	positive, err := renderOne(tmplText, vars)
	if err != nil {
		return nil, fmt.Errorf("prompt render: %w", err)
	}
	negative := ""
	if negativeTmpl != "" {
		negative, err = renderOne(negativeTmpl, vars)
		if err != nil {
			return nil, fmt.Errorf("negative prompt render: %w", err)
		}
	}
	if err := validate(positive); err != nil {
		return nil, err
	}
	return &RenderResult{
		PositivePrompt: positive,
		NegativePrompt: negative,
	}, nil
}

func renderOne(tmplText string, vars Vars) (string, error) {
	// 对用户输入进行 HTML 转义，防止模板注入
	safeVars := Vars{
		// 基础字段
		ProductName:         html.EscapeString(vars.ProductName),
		SellingPoints:       html.EscapeString(vars.SellingPoints),
		ImageTypeDesc:       vars.ImageTypeDesc,
		Platform:            vars.Platform,
		PlatformRules:       vars.PlatformRules,
		Language:            vars.Language,
		Ratio:               vars.Ratio,
		StyleDesc:           html.EscapeString(vars.StyleDesc),
		ReferenceImageCount: vars.ReferenceImageCount,
		// AI 视觉分析扩展字段（用户间接来源，需转义）
		ProductDescForPrompt: html.EscapeString(vars.ProductDescForPrompt),
		ProductType:          html.EscapeString(vars.ProductType),
		GarmentPosition:      html.EscapeString(vars.GarmentPosition),
		Color:                html.EscapeString(vars.Color),
		Material:             html.EscapeString(vars.Material),
		Style:                html.EscapeString(vars.Style),
		PrintDesign:          html.EscapeString(vars.PrintDesign),
		PrintDesignLock:      html.EscapeString(vars.PrintDesignLock),
		TargetAudience:       html.EscapeString(vars.TargetAudience),
		ProductStyle:         html.EscapeString(vars.ProductStyle),
		ProductNameZh:        html.EscapeString(vars.ProductNameZh),
		// 结构化卖点
		SP0Zh:     html.EscapeString(vars.SP0Zh),
		SP0En:     html.EscapeString(vars.SP0En),
		SP0ZhDesc: html.EscapeString(vars.SP0ZhDesc),
		SP0EnDesc: html.EscapeString(vars.SP0EnDesc),
		SP1Zh:     html.EscapeString(vars.SP1Zh),
		SP1En:     html.EscapeString(vars.SP1En),
		SP1ZhDesc: html.EscapeString(vars.SP1ZhDesc),
		SP1EnDesc: html.EscapeString(vars.SP1EnDesc),
		SP2Zh:     html.EscapeString(vars.SP2Zh),
		SP2En:     html.EscapeString(vars.SP2En),
		SP2ZhDesc: html.EscapeString(vars.SP2ZhDesc),
		SP2EnDesc: html.EscapeString(vars.SP2EnDesc),
		SP3Zh:     html.EscapeString(vars.SP3Zh),
		SP3En:     html.EscapeString(vars.SP3En),
		SP3ZhDesc: html.EscapeString(vars.SP3ZhDesc),
		SP3EnDesc: html.EscapeString(vars.SP3EnDesc),
		SP4Zh:     html.EscapeString(vars.SP4Zh),
		SP4En:     html.EscapeString(vars.SP4En),
		SP4ZhDesc: html.EscapeString(vars.SP4ZhDesc),
		SP4EnDesc: html.EscapeString(vars.SP4EnDesc),
		// 目标使用场景
		Scene0Zh: html.EscapeString(vars.Scene0Zh),
		Scene0En: html.EscapeString(vars.Scene0En),
		Scene1Zh: html.EscapeString(vars.Scene1Zh),
		Scene1En: html.EscapeString(vars.Scene1En),
		Scene2Zh: html.EscapeString(vars.Scene2Zh),
		Scene2En: html.EscapeString(vars.Scene2En),
	}
	t, err := template.New("prompt").Parse(tmplText)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, safeVars); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// validate 基础质量检查
func validate(prompt string) error {
	if len(prompt) < 10 {
		return fmt.Errorf("prompt too short: %d chars", len(prompt))
	}
	if len(prompt) > 4000 {
		return fmt.Errorf("prompt too long: %d chars", len(prompt))
	}
	// 违禁词检查（可从 DB 加载扩展）
	banned := []string{"fake certificate", "counterfeit", "medical cure"}
	lower := strings.ToLower(prompt)
	for _, w := range banned {
		if strings.Contains(lower, w) {
			return fmt.Errorf("prompt contains banned term: %q", w)
		}
	}
	return nil
}
