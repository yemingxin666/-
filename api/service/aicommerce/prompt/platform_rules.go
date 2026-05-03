package prompt

import (
	"geekai/store/model"
	"sync"
	"time"

	"gorm.io/gorm"
)

const platformRulesCacheTTL = 5 * time.Minute

type platformCacheItem struct {
	style     string
	expiresAt time.Time
}

var platformRulesCache sync.Map

// PlatformRules 返回各平台的风格规则描述，注入 Prompt 模板（带 5 分钟内存缓存）
func PlatformRules(db *gorm.DB, platform string) string {
	if platform == "" {
		platform = "generic"
	}
	if v, ok := platformRulesCache.Load(platform); ok {
		item := v.(platformCacheItem)
		if time.Now().Before(item.expiresAt) {
			return item.style
		}
		platformRulesCache.Delete(platform)
	}
	style := resolvePlatformStyle(db, platform)
	platformRulesCache.Store(platform, platformCacheItem{style: style, expiresAt: time.Now().Add(platformRulesCacheTTL)})
	return style
}

func resolvePlatformStyle(db *gorm.DB, platform string) string {
	if db != nil {
		if cfg, err := FindPlatformConfig(db, platform, true); err == nil && cfg.PromptStyle != "" {
			return cfg.PromptStyle
		}
		if cfg, err := FindPlatformConfig(db, "generic", true); err == nil && cfg.PromptStyle != "" {
			return cfg.PromptStyle
		}
	}
	return "通用电商风格：商品主体清晰，背景简洁，光线均衡"
}

// FindPlatformConfig 查询平台配置
func FindPlatformConfig(db *gorm.DB, value string, activeOnly bool) (*model.AiPlatformConfig, error) {
	var cfg model.AiPlatformConfig
	q := db.Where("value = ?", value)
	if activeOnly {
		q = q.Where("status = ?", model.PlatformStatusActive)
	}
	if err := q.First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ClearPlatformRulesCache 清空缓存（在 Admin 更新配置后调用）
func ClearPlatformRulesCache() {
	platformRulesCache.Range(func(k, _ interface{}) bool {
		platformRulesCache.Delete(k)
		return true
	})
}

// ImageTypeDesc 返回图片类型的中文描述，注入模板
func ImageTypeDesc(imageType string) string {
	descs := map[string]string{
		// 主图类型
		"traffic_cover":        "视觉冲击力强的引流封面，吸引用户点击",
		"core_selling_point":   "核心卖点展示图，清晰呈现产品核心优势",
		"scene_immersion":      "真实使用场景图，让用户产生代入感",
		"value_breakdown":      "价值拆解图，用图文展示产品价值点",
		"competitor_comparison": "竞品对比图，突出产品差异化优势",
		"detail_display":       "产品细节特写图，展示品质细节",
		"effect_proof":         "使用效果前后对比图，展示产品效果",
		"trust_building":       "信任背书图，展示资质证书、用户好评等",
		"final_push":           "促单图，展示限时优惠、赠品等促销信息",
		// 详情页类型
		"hero_visual":        "详情页首屏主视觉，第一眼吸引用户",
		"core_selling":       "核心卖点图文展示",
		"usage_scene":        "产品使用场景展示",
		"multi_angle":        "产品多角度全面展示",
		"atmosphere":         "营造使用氛围的场景图",
		"product_detail":     "产品材质和细节特写",
		"brand_story":        "品牌故事和理念展示",
		"size_capacity":      "产品尺寸、容量或尺码规格图",
		"effect_comparison":  "使用前后效果对比",
		"spec_reference":     "产品详细参数规格表",
		"craft_process":      "产品工艺和制作过程展示",
		"accessory_gift":     "包装内容和赠品展示",
		"series_showcase":    "系列产品展示",
		"ingredient":         "商品成分图",
		"after_sales":        "售后保障图",
		"usage_guide":        "使用建议图",
	}
	if d, ok := descs[imageType]; ok {
		return d
	}
	return imageType
}
