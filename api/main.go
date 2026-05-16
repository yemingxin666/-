package main

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"context"
	"embed"
	"geekai/core"
	"geekai/core/types"
	"geekai/handler"
	"geekai/handler/admin"
	handlerAicommerce "geekai/handler/aicommerce"
	logger2 "geekai/logger"
	"geekai/service"
	"geekai/service/aicommerce"
	aicWorker "geekai/service/aicommerce/worker"
	"geekai/service/oss"
	"geekai/service/payment"
	"geekai/service/sms"
	"geekai/store"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var logger = logger2.GetLogger()

//go:embed res
var xdbFS embed.FS

// AppLifecycle 应用程序生命周期
type AppLifecycle struct {
}

// OnStart 应用程序启动时执行
func (l *AppLifecycle) OnStart(context.Context) error {
	logger.Info("AppLifecycle OnStart")
	return nil
}

// OnStop 应用程序停止时执行
func (l *AppLifecycle) OnStop(context.Context) error {
	logger.Info("AppLifecycle OnStop")
	return nil
}

func NewAppLifeCycle() *AppLifecycle {
	return &AppLifecycle{}
}

func main() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.toml"
	}
	logger.Info("Loading config file: ", configFile)
	defer func() {
		if err := recover(); err != nil {
			logger.Error("Panic Error:", err)
			// 打印堆栈信息
			if os.Getenv("GEEKAI_DEBUG") == "true" {
				debug.PrintStack()
			}
		}
	}()

	app := fx.New(
		// 初始化配置应用配置
		fx.Provide(func() *types.AppConfig {
			config, err := core.LoadConfig(configFile)
			if err != nil {
				log.Fatal(err)
			}
			config.Path = configFile
			return config
		}),
		// 创建应用服务
		fx.Provide(core.NewServer),
		// 初始化
		fx.Invoke(func(s *core.AppServer, client *redis.Client) {
			s.Init(client)
		}),
		fx.Provide(func(db *gorm.DB) *types.SystemConfig {
			return core.LoadSystemConfig(db)
		}),

		// 初始化数据库
		fx.Provide(store.NewGormConfig),
		fx.Provide(store.NewMysql),
		fx.Provide(store.NewRedisClient),
		fx.Provide(store.NewLevelDB),

		fx.Provide(func() embed.FS {
			return xdbFS
		}),

		// 创建 Ip2Region 查询对象
		fx.Provide(func() (*xdb.Searcher, error) {
			file, err := xdbFS.Open("res/ip2region.xdb")
			if err != nil {
				return nil, err
			}
			cBuff, err := io.ReadAll(file)
			if err != nil {
				return nil, err
			}

			return xdb.NewWithBuffer(cBuff)
		}),

		// 创建控制器
		fx.Provide(handler.NewUserHandler),
		fx.Provide(handler.NewNetHandler),
		fx.Provide(handler.NewCaptchaHandler),
		fx.Provide(handler.NewSmsHandler),
		fx.Provide(handler.NewPaymentHandler),
		fx.Provide(handler.NewOrderHandler),
		fx.Provide(handler.NewProductHandler),
		fx.Provide(handler.NewConfigHandler),
		fx.Provide(handler.NewPowerLogHandler),

		fx.Provide(service.NewMigrationService),
		fx.Invoke(func(migrationService *service.MigrationService) {
			migrationService.StartMigrate()
		}),

		// 管理后台控制器
		fx.Provide(admin.NewConfigHandler),
		fx.Provide(admin.NewAdminHandler),
		fx.Provide(admin.NewUserHandler),
		fx.Provide(admin.NewRedeemHandler),
		fx.Provide(admin.NewDashboardHandler),
		fx.Provide(admin.NewProductHandler),
		fx.Provide(admin.NewOrderHandler),
		fx.Provide(admin.NewPowerLogHandler),

		// 邮件服务
		fx.Provide(service.NewSmtpService),
		// License 服务
		fx.Provide(service.NewLicenseService),
		fx.Invoke(func(licenseService *service.LicenseService) {
			licenseService.SyncLicense()
		}),

		fx.Provide(service.NewSnowflake),

		// 创建短信服务
		fx.Provide(sms.NewAliYunSmsService),
		fx.Provide(sms.NewBaoSmsService),
		fx.Provide(sms.NewSmsManager),
		fx.Provide(func(config *types.SystemConfig, client *redis.Client) (*service.CaptchaService, error) {
			return service.NewCaptchaService(config.Captcha, client)
		}),
		fx.Provide(func(config *types.SystemConfig, client *redis.Client) *service.WxLoginService {
			return service.NewWxLoginService(config.WxLogin, client)
		}),

		// 支付服务
		fx.Provide(payment.NewAlipayService),
		fx.Provide(payment.NewEPayService),
		fx.Provide(payment.NewWxpayService),

		// 文件上传服务
		fx.Provide(oss.NewLocalStorage),
		fx.Provide(oss.NewMiniOss),
		fx.Provide(oss.NewQiNiuOss),
		fx.Provide(oss.NewAliYunOss),
		fx.Provide(oss.NewUploaderManager),

		// 用户服务
		fx.Provide(service.NewUserService),

		// 注册路由
		fx.Invoke(func(s *core.AppServer, h *handler.UserHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.NetHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.CaptchaHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.SmsHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.ConfigHandler) {
			h.RegisterRoutes()
		}),

		// 管理后台路由注册
		fx.Invoke(func(s *core.AppServer, h *admin.ConfigHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.ManagerHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.UserHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.RedeemHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.DashboardHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.PaymentHandler) {
			h.RegisterRoutes()
			h.StartSyncOrders()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.ProductHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.OrderHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.OrderHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *handler.ProductHandler) {
			h.RegisterRoutes()
		}),

		fx.Provide(handler.NewInviteHandler),
		fx.Invoke(func(s *core.AppServer, h *handler.InviteHandler) {
			h.RegisterRoutes()
		}),

		fx.Provide(admin.NewUploadHandler),
		fx.Invoke(func(s *core.AppServer, h *admin.UploadHandler) {
			h.RegisterRoutes()
		}),

		fx.Invoke(func(s *core.AppServer, h *handler.PowerLogHandler) {
			h.RegisterRoutes()
		}),
		fx.Invoke(func(s *core.AppServer, h *admin.PowerLogHandler) {
			h.RegisterRoutes()
		}),
		fx.Provide(admin.NewMenuHandler),
		fx.Invoke(func(s *core.AppServer, h *admin.MenuHandler) {
			h.RegisterRoutes()
		}),
		fx.Provide(handler.NewMenuHandler),
		fx.Invoke(func(s *core.AppServer, h *handler.MenuHandler) {
			h.RegisterRoutes()
		}),

		fx.Invoke(func(s *core.AppServer, db *gorm.DB) {
			go func() {
				err := s.Run(db)
				if err != nil {
					logger.Error(err)
					os.Exit(0)
				}
			}()
		}),
		fx.Provide(NewAppLifeCycle),
		// 注册生命周期回调函数
		fx.Invoke(func(lifecycle fx.Lifecycle, lc *AppLifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return lc.OnStart(ctx)
				},
				OnStop: func(ctx context.Context) error {
					return lc.OnStop(ctx)
				},
			})
		}),

		// 电商 AI 生图模块
		fx.Provide(admin.NewAiCommerceHandler),
		fx.Invoke(func(s *core.AppServer, h *admin.AiCommerceHandler) {
			h.RegisterRoutes()
		}),
		fx.Provide(func(app *core.AppServer, db *gorm.DB, rdb *redis.Client, mgr *oss.UploaderManager) *handlerAicommerce.ImageHandler {
			cfg := buildAiCommerceConfig(app.Config)
			svc := aicommerce.NewImageService(db, rdb, cfg, mgr.GetUploadHandler())
			return handlerAicommerce.NewImageHandler(app, db, svc, mgr)
		}),
		fx.Invoke(func(s *core.AppServer, h *handlerAicommerce.ImageHandler) {
			h.RegisterRoutes()
		}),
		fx.Provide(func(app *core.AppServer, db *gorm.DB, rdb *redis.Client, mgr *oss.UploaderManager) *aicWorker.Dispatcher {
			cfg := buildAiCommerceConfig(app.Config)
			return aicWorker.NewDispatcher(db, rdb, cfg, mgr)
		}),
		fx.Invoke(func(lc fx.Lifecycle, d *aicWorker.Dispatcher) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go d.Run(context.Background())
					return nil
				},
			})
		}),
		fx.Provide(func(db *gorm.DB, mgr *oss.UploaderManager) *aicWorker.ReferenceCleaner {
			return aicWorker.NewReferenceCleaner(db, mgr)
		}),
		fx.Invoke(func(lc fx.Lifecycle, c *aicWorker.ReferenceCleaner) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go c.Run(context.Background())
					return nil
				},
			})
		}),
	)
	// 启动应用程序
	go func() {
		if err := app.Start(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 关闭应用程序
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		log.Fatal(err)
	}

}

// buildAiCommerceConfig 从 AppConfig 构造电商生图模块运行配置。
//
// 为什么要做这一层合并：
//   - aicommerce.Config 原本各路默认值只在 DefaultConfig() 里兜底，
//     直接 DefaultConfig() 会丢 config.toml [AiCommerce] 节里填写的凭证；
//   - OSSBucket 一定要有值，否则 white_bg chain 的参考图 URL 拼不出来；
//     这里优先用 [AiCommerce] 节的 oss_bucket，没有时回退到 [OSS.AliYun].Bucket。
func buildAiCommerceConfig(app *types.AppConfig) aicommerce.Config {
	cfg := aicommerce.DefaultConfig()
	ac := app.AiCommerce

	// 覆盖默认值；空字符串表示用户没填，走 DefaultConfig 的兜底
	if ac.SiliconFlowBaseURL != "" {
		cfg.SiliconFlowBaseURL = ac.SiliconFlowBaseURL
	}
	if ac.SiliconFlowAPIKey != "" {
		cfg.SiliconFlowAPIKey = ac.SiliconFlowAPIKey
	}
	if ac.SiliconFlowModel != "" {
		cfg.SiliconFlowModel = ac.SiliconFlowModel
	}
	if ac.TongyiBaseURL != "" {
		cfg.TongyiBaseURL = ac.TongyiBaseURL
	}
	if ac.TongyiAPIKey != "" {
		cfg.TongyiAPIKey = ac.TongyiAPIKey
	}
	if ac.TongyiModel != "" {
		cfg.TongyiModel = ac.TongyiModel
	}
	if ac.BaiduOCRAppID != "" {
		cfg.BaiduOCRAppID = ac.BaiduOCRAppID
	}
	if ac.BaiduOCRAPIKey != "" {
		cfg.BaiduOCRAPIKey = ac.BaiduOCRAPIKey
	}
	if ac.BaiduOCRSecretKey != "" {
		cfg.BaiduOCRSecretKey = ac.BaiduOCRSecretKey
	}
	if ac.BaiduTranslateAppID != "" {
		cfg.BaiduTranslateAppID = ac.BaiduTranslateAppID
	}
	if ac.BaiduTranslateSecret != "" {
		cfg.BaiduTranslateSecret = ac.BaiduTranslateSecret
	}
	if ac.AliyunVisionAccessKeyID != "" {
		cfg.AliyunVisionAccessKeyID = ac.AliyunVisionAccessKeyID
	}
	if ac.AliyunVisionAccessKeySecret != "" {
		cfg.AliyunVisionAccessKeySecret = ac.AliyunVisionAccessKeySecret
	}
	if ac.AliyunVisionRegion != "" {
		cfg.AliyunVisionRegion = ac.AliyunVisionRegion
	}
	// 跨区域中转：仅在显式启用时透传
	cfg.AliyunVisionRelayEnabled = ac.AliyunVisionRelayEnabled
	if ac.AliyunVisionRelayEndpoint != "" {
		cfg.AliyunVisionRelayEndpoint = ac.AliyunVisionRelayEndpoint
	}
	if ac.AliyunVisionRelayAccessKey != "" {
		cfg.AliyunVisionRelayAccessKey = ac.AliyunVisionRelayAccessKey
	}
	if ac.AliyunVisionRelayAccessSecret != "" {
		cfg.AliyunVisionRelayAccessSecret = ac.AliyunVisionRelayAccessSecret
	}
	if ac.AliyunVisionRelayBucket != "" {
		cfg.AliyunVisionRelayBucket = ac.AliyunVisionRelayBucket
	}
	if ac.AliyunVisionRelayPrefix != "" {
		cfg.AliyunVisionRelayPrefix = ac.AliyunVisionRelayPrefix
	}
	if ac.QueueName != "" {
		cfg.QueueName = ac.QueueName
	}
	if ac.WorkerConcurrency > 0 {
		cfg.WorkerConcurrency = ac.WorkerConcurrency
	}
	if ac.AssetURLTTL > 0 {
		cfg.AssetURLTTL = ac.AssetURLTTL
	}

	// OSSBucket：优先 [AiCommerce] 显式配置，回退到 [OSS.AliYun]
	if ac.OSSBucket != "" {
		cfg.OSSBucket = ac.OSSBucket
	} else if app.OSS.AliYun.Bucket != "" {
		cfg.OSSBucket = app.OSS.AliYun.Bucket
	}

	return cfg
}
