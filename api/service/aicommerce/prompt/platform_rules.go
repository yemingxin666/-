package prompt

// PlatformRules 返回各平台的风格规则描述，注入 Prompt 模板
func PlatformRules(platform string) string {
	rules := map[string]string{
		"taobao":   "淘宝平台风格：色彩鲜艳、信息密集、突出促销信息、中文排版清晰",
		"tmall":    "天猫平台风格：品质感强、简洁大气、突出品牌调性",
		"jd":       "京东平台风格：科技感、蓝色系、突出品质与服务",
		"pinduoduo": "拼多多平台风格：价格醒目、红色系、强调性价比",
		"tiktok":   "抖音平台风格：视觉冲击强、竖版构图、适合短视频封面",
		"xiaohongshu": "小红书平台风格：生活感强、清新自然、适合种草内容",
		"amazon":   "Amazon style: clean white background, professional product photography, no text overlay on main image",
		"shopee":   "Shopee style: bright colors, mobile-first layout, Southeast Asia market preference",
		"lazada":   "Lazada style: clean product display, Southeast Asia e-commerce standard",
		"aliexpress": "AliExpress style: international audience, clear product details, neutral background",
		"generic":  "通用电商风格：商品主体清晰，背景简洁，光线均匀",
	}
	if r, ok := rules[platform]; ok {
		return r
	}
	return rules["generic"]
}

// ImageTypeDesc 返回图片类型的中文描述，注入模板
func ImageTypeDesc(imageType string) string {
	descs := map[string]string{
		// 主图类型
		"引流封面":   "视觉冲击力强的引流封面，吸引用户点击",
		"核心卖点":   "核心卖点展示图，清晰呈现产品核心优势",
		"场景代入":   "真实使用场景图，让用户产生代入感",
		"价值拆解":   "价值拆解图，用图文展示产品价值点",
		"竞品对比":   "竞品对比图，突出产品差异化优势",
		"细节展示":   "产品细节特写图，展示品质细节",
		"效果证明":   "使用效果前后对比图，展示产品效果",
		"信任消疑":   "信任背书图，展示资质证书、用户好评等",
		"临门一脚":   "促单图，展示限时优惠、赠品等促销信息",
		// 详情页类型
		"首屏主视觉":    "详情页首屏主视觉，第一眼吸引用户",
		"核心卖点图":    "核心卖点图文展示",
		"使用场景图":    "产品使用场景展示",
		"多角度图":     "产品多角度全面展示",
		"场景氛围图":    "营造使用氛围的场景图",
		"商品细节图":    "产品材质和细节特写",
		"品牌故事图":    "品牌故事和理念展示",
		"尺寸容量尺码图": "产品尺寸、容量或尺码规格图",
		"效果对比图":    "使用前后效果对比",
		"详细规格参数表": "产品详细参数规格表",
		"工艺制作图":    "产品工艺和制作过程展示",
		"配件赠品图":    "包装内容和赠品展示",
	}
	if d, ok := descs[imageType]; ok {
		return d
	}
	return imageType
}
