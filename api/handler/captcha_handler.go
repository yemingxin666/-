package handler

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
	"geekai/service"
	"geekai/utils/resp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type CaptchaHandler struct {
	App     *core.AppServer
	redis   *redis.Client
	service *service.CaptchaService
}

func NewCaptchaHandler(app *core.AppServer, s *service.CaptchaService, rdb *redis.Client, sysConfig *types.SystemConfig) *CaptchaHandler {
	return &CaptchaHandler{App: app, service: s, redis: rdb}
}

func (h *CaptchaHandler) RegisterRoutes() {
	group := h.App.Engine.Group("/api/captcha/")

	group.GET("slide/get", middleware.RateLimitEvery(h.redis, 2*time.Second), h.SlideGet)
	group.GET("config", h.GetConfig)
}

func (h *CaptchaHandler) GetConfig(c *gin.Context) {
	resp.SUCCESS(c, gin.H{"enabled": h.service.GetConfig().Enabled, "type": "slide"})
}

func (h *CaptchaHandler) SlideGet(c *gin.Context) {
	if !h.service.GetConfig().Enabled {
		resp.ERROR(c, "验证码服务未启用")
		return
	}

	data, err := h.service.SlideGet()
	if err != nil {
		logger.Error(err)
		resp.ERROR(c, "验证码生成失败，请稍后重试")
		return
	}

	resp.SUCCESS(c, data)
}

