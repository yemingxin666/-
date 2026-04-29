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
	ProductName        string
	SellingPoints      string
	ImageTypeDesc      string
	Platform           string
	PlatformRules      string
	Language           string
	Ratio              string
	StyleDesc          string
	ReferenceImageCount int
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
		ProductName:         html.EscapeString(vars.ProductName),
		SellingPoints:       html.EscapeString(vars.SellingPoints),
		ImageTypeDesc:       vars.ImageTypeDesc,       // 系统控制，无需转义
		Platform:            vars.Platform,
		PlatformRules:       vars.PlatformRules,
		Language:            vars.Language,
		Ratio:               vars.Ratio,
		StyleDesc:           html.EscapeString(vars.StyleDesc),
		ReferenceImageCount: vars.ReferenceImageCount,
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
