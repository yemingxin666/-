package service

// ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// Copyright 2023 The Geek-AI Authors. All rights reserved.
// Use of this source code is governed by a Apache-2.0 license
// that can be found in the LICENSE file.
// @Author yangjian102621@163.com
// ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"context"
	"encoding/json"
	"fmt"
	"geekai/core/types"
	"geekai/store"
	"geekai/store/model"
	"strings"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 迁移状态Redis key
	MigrationStatusKey = "config_migration:status"
	// 迁移完成标志
	MigrationCompleted = "completed"
)

// MigrationService 配置迁移服务
type MigrationService struct {
	db             *gorm.DB
	redisClient    *redis.Client
	appConfig      *types.AppConfig
	levelDB        *store.LevelDB
	licenseService *LicenseService
}

func NewMigrationService(db *gorm.DB, redisClient *redis.Client, appConfig *types.AppConfig, levelDB *store.LevelDB, licenseService *LicenseService) *MigrationService {
	return &MigrationService{
		db:             db,
		redisClient:    redisClient,
		appConfig:      appConfig,
		levelDB:        levelDB,
		licenseService: licenseService,
	}
}

func (s *MigrationService) StartMigrate() {
	go func() {
		s.MigrateConfig(s.appConfig)
		s.TableMigration()
		s.MigrateLicense()
	}()
}

// 迁移 License
func (s *MigrationService) MigrateLicense() {
	key := "migrate:license"
	if s.redisClient.Get(context.Background(), key).Val() == "1" {
		logger.Info("License 已迁移，跳过迁移")
		return
	}

	logger.Info("开始迁移 License...")
	var license types.License
	err := s.levelDB.Get(types.LicenseKey, &license)
	if err != nil {
		license = types.License{
			Key:       "",
			MachineId: "",
			Configs:   types.LicenseConfig{UserNum: 0, DeCopy: false},
			ExpiredAt: 0,
			IsActive:  false,
		}
	}
	logger.Infof("迁移 License: %+v", license)
	if err := s.saveConfig(types.ConfigKeyLicense, license); err != nil {
		logger.Errorf("迁移 License 失败: %v", err)
		return
	}
	s.licenseService.SetLicense(license.Key)
	logger.Info("迁移 License 完成")
	s.redisClient.Set(context.Background(), key, "1", 0)
}

// 迁移配置内容
func (s *MigrationService) MigrateConfigContent() error {
	// 用户协议
	if err := s.saveConfig(types.ConfigKeyPrivacy, map[string]string{
		"content": "用户协议内容",
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}
	// 隐私政策
	if err := s.saveConfig(types.ConfigKeyAgreement, map[string]string{
		"content": "隐私政策内容",
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}
	// 思维导图
	if err := s.saveConfig(types.ConfigKeyMarkMap, map[string]string{
		"content": `# GeekAI 演示站

- 完整的开源系统，前端应用和后台管理系统皆可开箱即用。
- 基于 Websocket 实现，完美的打字机体验。
- 内置了各种预训练好的角色应用,轻松满足你的各种聊天和应用需求。
- 支持 OPenAI，Azure，文心一言，讯飞星火，清华 ChatGLM等多个大语言模型。
- 支持 MidJourney / Stable Diffusion AI 绘画集成，开箱即用。
- 支持使用个人微信二维码作为充值收费的支付渠道，无需企业支付通道。
- 已集成支付宝支付功能，微信支付，支持多种会员套餐和点卡购买功能。
- 集成插件 API 功能，可结合大语言模型的 function 功能开发各种强大的插件。`,
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}

	// 微信登录配置
	if err := s.saveConfig(types.ConfigKeyWxLogin, map[string]string{
		"api_key":    "",
		"notify_url": "",
		"enabled":    "false",
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}

	// 验证码配置
	if err := s.saveConfig(types.ConfigKeyCaptcha, map[string]string{
		"api_key": "",
		"type":    "dot",
		"enabled": "false",
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}

	// 文本审核
	if err := s.saveConfig(types.ConfigKeyModeration, map[string]any{
		"enable":       "false",
		"active":       "gitee",
		"enable_guide": "false",
		"guide_prompt": "",
		"gitee": map[string]string{
			"api_key": "",
			"model":   "Security-semantic-filtering",
		},
		"baidu": map[string]string{
			"access_key": "",
			"secret_key": "",
		},
		"tencent": map[string]string{
			"access_key": "",
			"secret_key": "",
		},
	}); err != nil {
		return fmt.Errorf("迁移配置内容失败: %v", err)
	}

	return nil
}

// 数据表迁移
func (s *MigrationService) TableMigration() {
	// 新数据表
	s.db.AutoMigrate(&model.Moderation{})

	// 订单字段整理
	if s.db.Migrator().HasColumn(&model.Order{}, "pay_type") {
		s.db.Migrator().RenameColumn(&model.Order{}, "pay_type", "channel")
	}
	if !s.db.Migrator().HasColumn(&model.Order{}, "checked") {
		s.db.Migrator().AddColumn(&model.Order{}, "checked")
	}

	// 重命名 config 表字段
	if s.db.Migrator().HasColumn(&model.Config{}, "config_json") {
		s.db.Migrator().RenameColumn(&model.Config{}, "config_json", "value")
	}
	if s.db.Migrator().HasColumn(&model.Config{}, "marker") {
		s.db.Migrator().RenameColumn(&model.Config{}, "marker", "name")
	}
	if s.db.Migrator().HasIndex(&model.Config{}, "idx_chatgpt_configs_key") {
		s.db.Migrator().DropIndex(&model.Config{}, "idx_chatgpt_configs_key")
	}
	if s.db.Migrator().HasIndex(&model.Config{}, "marker") {
		s.db.Migrator().DropIndex(&model.Config{}, "marker")
	}

	// 手动删除字段
	if s.db.Migrator().HasColumn(&model.Order{}, "deleted_at") {
		s.db.Migrator().DropColumn(&model.Order{}, "deleted_at")
	}
	if s.db.Migrator().HasColumn(&model.ChatItem{}, "deleted_at") {
		s.db.Migrator().DropColumn(&model.ChatItem{}, "deleted_at")
	}
	if s.db.Migrator().HasColumn(&model.ChatMessage{}, "deleted_at") {
		s.db.Migrator().DropColumn(&model.ChatMessage{}, "deleted_at")
	}
	if s.db.Migrator().HasColumn(&model.User{}, "chat_config") {
		s.db.Migrator().DropColumn(&model.User{}, "chat_config")
	}
	if s.db.Migrator().HasColumn(&model.ChatModel{}, "category") {
		s.db.Migrator().DropColumn(&model.ChatModel{}, "category")
	}
	if s.db.Migrator().HasColumn(&model.ChatModel{}, "description") {
		s.db.Migrator().DropColumn(&model.ChatModel{}, "description")
	}
	if s.db.Migrator().HasColumn(&model.Product{}, "discount") {
		s.db.Migrator().DropColumn(&model.Product{}, "discount")
	}
	if s.db.Migrator().HasColumn(&model.Product{}, "days") {
		s.db.Migrator().DropColumn(&model.Product{}, "days")
	}
	if s.db.Migrator().HasColumn(&model.Product{}, "app_url") {
		s.db.Migrator().DropColumn(&model.Product{}, "app_url")
	}
	if s.db.Migrator().HasColumn(&model.Product{}, "url") {
		s.db.Migrator().DropColumn(&model.Product{}, "url")
	}
	if err := s.db.AutoMigrate(&model.AiPlatformConfig{}); err != nil {
		logger.Errorf("migrate ai platform configs failed: %v", err)
		return
	}
	if err := s.SeedAiPlatformConfigs(); err != nil {
		logger.Errorf("seed ai platform configs failed: %v", err)
	}
}

func (s *MigrationService) SeedAiPlatformConfigs() error {
	items := []model.AiPlatformConfig{
		{
			Value: "pinduoduo", Label: "拼多多", DefaultLanguage: "zh-CN", DefaultRatio: "1:1", SortOrder: 10,
			PromptStyle: "拼多多平台风格：价格醒目、红色系、强调性价比和促销氛围，商品主体清晰突出",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "taobao", Label: "淘宝", DefaultLanguage: "zh-CN", DefaultRatio: "1:1", SortOrder: 20,
			PromptStyle: "淘宝平台风格：色彩鲜明、信息清晰、突出促销信息和商品卖点，中文排版清楚",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "tmall", Label: "天猫", DefaultLanguage: "zh-CN", DefaultRatio: "1:1", SortOrder: 30,
			PromptStyle: "天猫平台风格：品质感强、简洁大气、突出品牌调性和高端质感",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "jd", Label: "京东", DefaultLanguage: "zh-CN", DefaultRatio: "1:1", SortOrder: 40,
			PromptStyle: "京东平台风格：科技感、可信赖、突出品质、服务保障和专业商品展示",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "douyin", Label: "抖音", DefaultLanguage: "zh-CN", DefaultRatio: "9:16", SortOrder: 50,
			PromptStyle: "抖音平台风格：视觉冲击强、年轻化、适合短视频电商场景，突出第一眼吸引力",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "xiaohongshu", Label: "小红书", DefaultLanguage: "zh-CN", DefaultRatio: "3:4", SortOrder: 60,
			PromptStyle: "小红书平台风格：生活感强、清新自然、真实种草氛围，适合内容化商品展示",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "amazon", Label: "Amazon", DefaultLanguage: "en-US", DefaultRatio: "1:1", SortOrder: 70,
			PromptStyle:    "Amazon style: clean white background, professional product photography, accurate product representation, no text overlay on main image",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{"force_white_bg": true, "no_text_overlay": true},
		},
		{
			Value: "shopee", Label: "Shopee", DefaultLanguage: "en-US", DefaultRatio: "1:1", SortOrder: 80,
			PromptStyle:    "Shopee style: bright and mobile-first product display for Southeast Asia marketplace, clear product focus",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{"force_white_bg": true, "no_text_overlay": true},
		},
		{
			Value: "shopify", Label: "Shopify", DefaultLanguage: "en-US", DefaultRatio: "1:1", SortOrder: 90,
			PromptStyle:    "Shopify style: brand-owned storefront, clean lifestyle presentation, conversion-focused product photography",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "lazada", Label: "Lazada", DefaultLanguage: "en-US", DefaultRatio: "1:1", SortOrder: 100,
			PromptStyle:    "Lazada style: clean product display, Southeast Asia e-commerce standard, clear and trustworthy product presentation",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{"force_white_bg": true, "no_text_overlay": true},
		},
		{
			Value: "aliexpress", Label: "AliExpress", DefaultLanguage: "en-US", DefaultRatio: "1:1", SortOrder: 110,
			PromptStyle:    "AliExpress style: international audience, clear product details, neutral background, cross-border marketplace friendly",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
		{
			Value: "generic", Label: "通用", DefaultLanguage: "zh-CN", DefaultRatio: "1:1", SortOrder: 999,
			PromptStyle:    "通用电商风格：商品主体清晰，背景简洁，光线均衡，突出商品质感与核心卖点",
			PriorityImages: model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}},
			Constraints:    model.JSONMap{},
		},
	}
	for i := range items {
		items[i].Status = model.PlatformStatusActive
	}
	return s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "value"}}, DoNothing: true}).Create(&items).Error
}

// 迁移配置数据
func (s *MigrationService) MigrateConfig(config *types.AppConfig) error {

	logger.Info("开始迁移配置到数据库...")

	// 迁移支付配置
	if err := s.migratePaymentConfig(config); err != nil {
		logger.Errorf("迁移支付配置失败: %v", err)
		return err
	}

	// 迁移存储配置
	if err := s.migrateStorageConfig(config); err != nil {
		logger.Errorf("迁移存储配置失败: %v", err)
		return err
	}

	// 迁移通信配置
	if err := s.migrateCommunicationConfig(config); err != nil {
		logger.Errorf("迁移通信配置失败: %v", err)
		return err
	}

	// 迁移配置内容
	if err := s.MigrateConfigContent(); err != nil {
		logger.Errorf("迁移配置内容失败: %v", err)
		return err
	}

	logger.Info("配置迁移完成")
	return nil
}

// 迁移支付配置
func (s *MigrationService) migratePaymentConfig(config *types.AppConfig) error {

	paymentConfig := types.PaymentConfig{
		Alipay: config.AlipayConfig,
		Epay:   config.GeekPayConfig,
		WxPay:  config.WechatPayConfig,
	}
	if err := s.saveConfig(types.ConfigKeyPayment, paymentConfig); err != nil {
		return err
	}

	return nil
}

// 迁移存储配置
func (s *MigrationService) migrateStorageConfig(config *types.AppConfig) error {

	ossConfig := types.OSSConfig{
		Active: config.OSS.Active,
		Local:  config.OSS.Local,
		Minio:  config.OSS.Minio,
		QiNiu:  config.OSS.QiNiu,
		AliYun: config.OSS.AliYun,
	}
	return s.saveConfig(types.ConfigKeyOss, ossConfig)
}

// 迁移通信配置
func (s *MigrationService) migrateCommunicationConfig(config *types.AppConfig) error {
	// SMTP配置
	smtpConfig := map[string]any{
		"use_tls":  config.SmtpConfig.UseTls,
		"host":     config.SmtpConfig.Host,
		"port":     config.SmtpConfig.Port,
		"app_name": config.SmtpConfig.AppName,
		"from":     config.SmtpConfig.From,
		"password": config.SmtpConfig.Password,
	}
	if err := s.saveConfig(types.ConfigKeySmtp, smtpConfig); err != nil {
		return err
	}

	// 短信配置
	smsConfig := map[string]any{
		"active": strings.ToLower(config.SMS.Active),
		"aliyun": map[string]any{
			"access_key":    config.SMS.Ali.AccessKey,
			"access_secret": config.SMS.Ali.AccessSecret,
			"sign":          config.SMS.Ali.Sign,
			"code_temp_id":  config.SMS.Ali.CodeTempId,
		},
		"bao": map[string]any{
			"username":      config.SMS.Bao.Username,
			"password":      config.SMS.Bao.Password,
			"sign":          config.SMS.Bao.Sign,
			"code_template": config.SMS.Bao.CodeTemplate,
		},
	}
	return s.saveConfig(types.ConfigKeySms, smsConfig)
}

// 保存配置到数据库
func (s *MigrationService) saveConfig(key string, config any) error {
	// 检查是否已存在
	var existingConfig model.Config
	if err := s.db.Where("name", key).First(&existingConfig).Error; err == nil {
		// 配置已存在，跳过
		logger.Infof("配置 %s 已存在，跳过迁移", key)
		return nil
	}

	// 序列化配置
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	// 保存到数据库
	newConfig := model.Config{
		Name:  key,
		Value: string(configJSON),
	}
	if err := s.db.Create(&newConfig).Error; err != nil {
		return err
	}

	logger.Infof("成功迁移配置 %s", key)
	return nil
}
