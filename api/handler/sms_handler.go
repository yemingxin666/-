package handler

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"fmt"
	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
	"geekai/service"
	"geekai/service/sms"
	"geekai/store/model"
	"geekai/utils"
	"geekai/utils/resp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const CodeStorePrefix = "/verify/codes/"

const (
	smsCodeTTL           = 5 * time.Minute
	smsSendCooldown      = 60 * time.Second
	smsMaxVerifyAttempts = 5
)

const (
	SmsSceneRegister   = "register"
	SmsSceneResetPass  = "reset_pass"
	SmsSceneBindMobile = "bind_mobile"
	SmsSceneBindEmail  = "bind_email"
)

func smsCodeKey(scene, receiver string) string {
	return CodeStorePrefix + scene + "/" + receiver
}

func smsAttemptKey(scene, receiver string) string {
	return "/verify/code-attempts/" + scene + "/" + receiver
}

func smsCooldownKey(scene, receiver string) string {
	return fmt.Sprintf("sms:cooldown:%s:%s", scene, receiver)
}

func normalizeScene(scene string) (string, bool) {
	switch scene {
	case SmsSceneRegister, SmsSceneResetPass, SmsSceneBindMobile, SmsSceneBindEmail:
		return scene, true
	default:
		return "", false
	}
}

type SmsHandler struct {
	BaseHandler
	redis          *redis.Client
	sms            *sms.SmsManager
	smtp           *service.SmtpService
	captchaService *service.CaptchaService
}

func NewSmsHandler(
	app *core.AppServer,
	client *redis.Client,
	sms *sms.SmsManager,
	smtp *service.SmtpService,
	captcha *service.CaptchaService) *SmsHandler {
	return &SmsHandler{
		redis:          client,
		sms:            sms,
		captchaService: captcha,
		smtp:           smtp,
		BaseHandler:    BaseHandler{App: app}}
}

// RegisterRoutes 注册路由
func (h *SmsHandler) RegisterRoutes() {
	group := h.App.Engine.Group("/api/sms/")
	group.POST("code", middleware.RateLimitEvery(h.redis, smsSendCooldown), h.SendCode)
}

// SendCode 发送验证码
func (h *SmsHandler) SendCode(c *gin.Context) {
	var data struct {
		Receiver string `json:"receiver"`
		Scene    string `json:"scene"`
		Key      string `json:"key"`
		Dots     string `json:"dots,omitempty"`
		X        int    `json:"x,omitempty"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	scene, ok := normalizeScene(data.Scene)
	if !ok {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	if h.captchaService.GetConfig().Enabled {
		var check bool
		if data.X != 0 {
			check = h.captchaService.SlideCheck(data)
		} else {
			check = h.captchaService.Check(data)
		}
		if !check {
			resp.ERROR(c, "请先完成人机验证")
			return
		}
	}

	receiver := strings.TrimSpace(data.Receiver)
	var isEmail bool
	if utils.IsValidEmail(receiver) {
		isEmail = true
		receiver = strings.ToLower(receiver)
	} else if utils.IsValidMobile(receiver) {
		isEmail = false
	} else {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	if isEmail {
		if !utils.Contains(h.App.SysConfig.Base.RegisterWays, "email") {
			resp.ERROR(c, "该注册方式暂不可用")
			return
		}
		if len(h.App.SysConfig.Base.EmailWhiteList) > 0 {
			inWhiteList := false
			for _, suffix := range h.App.SysConfig.Base.EmailWhiteList {
				if strings.HasSuffix(receiver, suffix) {
					inWhiteList = true
					break
				}
			}
			if !inWhiteList {
				resp.ERROR(c, "该注册方式暂不可用")
				return
			}
		}
	} else {
		if !utils.Contains(h.App.SysConfig.Base.RegisterWays, "mobile") {
			resp.ERROR(c, "该注册方式暂不可用")
			return
		}
	}

	if scene == SmsSceneRegister {
		var count int64
		if isEmail {
			h.DB.Model(&model.User{}).Where("email = ?", receiver).Count(&count)
		} else {
			h.DB.Model(&model.User{}).Where("mobile = ?", receiver).Count(&count)
		}
		if count > 0 {
			resp.ERROR(c, "该账号已注册，请直接登录")
			return
		}
	}

	cooldownKey := smsCooldownKey(scene, receiver)
	cdOk, err := h.redis.SetNX(c, cooldownKey, 1, smsSendCooldown).Result()
	if err != nil {
		resp.ERROR(c, "验证码发送失败，请稍后再试")
		return
	}
	if !cdOk {
		resp.ERROR(c, "请求过于频繁，请稍后再试")
		return
	}

	code := utils.SecureRandomNumber(6)
	codeKey := smsCodeKey(scene, receiver)
	_, err = h.redis.Set(c, codeKey, code, smsCodeTTL).Result()
	if err != nil {
		_ = h.redis.Del(c, cooldownKey).Err()
		resp.ERROR(c, "验证码保存失败")
		return
	}

	if isEmail {
		err = h.smtp.SendVerifyCode(receiver, code)
	} else {
		err = h.sms.GetService().SendVerifyCode(receiver, code)
	}
	if err != nil {
		_ = h.redis.Del(c, cooldownKey, codeKey).Err()
		logger.Warnf("发送验证码失败: %v", err)
		resp.ERROR(c, "验证码发送失败，请稍后再试")
		return
	}

	resp.SUCCESS(c)
}
