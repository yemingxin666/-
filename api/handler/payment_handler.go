package handler

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"embed"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
	"geekai/service"
	"geekai/service/payment"
	"geekai/store/model"
	"geekai/utils"
	"geekai/utils/resp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PayWay struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PaymentHandler 支付服务回调 handler
type PaymentHandler struct {
	BaseHandler
	alipayService *payment.AlipayService
	epayService   *payment.EPayService
	wxpayService  *payment.WxPayService
	snowflake     *service.Snowflake
	userService   *service.UserService
	fs            embed.FS
	config        *types.PaymentConfig
}

func NewPaymentHandler(
	server *core.AppServer,
	alipayService *payment.AlipayService,
	geekPayService *payment.EPayService,
	wxpayService *payment.WxPayService,
	db *gorm.DB,
	userService *service.UserService,
	snowflake *service.Snowflake,
	fs embed.FS,
	sysConfig *types.SystemConfig) *PaymentHandler {
	return &PaymentHandler{
		alipayService: alipayService,
		epayService:   geekPayService,
		wxpayService:  wxpayService,
		snowflake:     snowflake,
		userService:   userService,
		fs:            fs,
		BaseHandler: BaseHandler{
			App: server,
			DB:  db,
		},
		config: &sysConfig.Payment,
	}
}

// RegisterRoutes 注册路由
func (h *PaymentHandler) RegisterRoutes() {
	rg := h.App.Engine.Group("/api/payment/")

	// 支付回调接口（公开）
	rg.POST("notify/alipay", h.AlipayNotify)
	rg.GET("notify/epay", h.EPayNotify)
	rg.POST("notify/wxpay", h.WxpayNotify)

	// 需要用户登录的接口
	rg.Use(middleware.UserAuthMiddleware(h.App.Config.Session.SecretKey, h.App.Redis))
	{
		rg.POST("create", h.CreateOrder)
	}
}

func (h *PaymentHandler) StartSyncOrders() {
	go func() {
		for {
			err := h.SyncOrders()
			if err != nil {
				logger.Error(err)
			}
			time.Sleep(time.Second * 5)
		}
	}()
}

// SyncOrders 同步订单状态
func (h *PaymentHandler) SyncOrders() error {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("同步订单状态发生异常: %v", err)
		}
	}()
	var orders []model.Order
	err := h.DB.Where("status", types.OrderNotPaid).Where("checked", false).Find(&orders).Error
	if err != nil {
		return err
	}

	for _, order := range orders {
		time.Sleep(time.Second * 1)
		//超时15分钟的订单，直接标记为已关闭
		if time.Now().After(order.CreatedAt.Add(time.Minute * 5)) {
			h.DB.Model(&model.Order{}).Where("id", order.Id).Update("checked", true)
			logger.Errorf("订单超时：%v", order)
			continue
		}
		// 查询订单状态
		var res payment.OrderInfo
		switch order.Channel {
		case payment.PayChannelEpay:
			res, err = h.epayService.Query(order.OrderNo)
			if err != nil {
				logger.Errorf("error with query order info: %v", err)
				continue
			}
			// 微信支付
		case payment.PayChannelWX:
			res, err = h.wxpayService.Query(order.OrderNo)
			logger.Debugf("微信支付订单状态：%+v", res)
			if err != nil {
				logger.Errorf("error with query order info: %v", err)
				continue
			}
		case payment.PayChannelAL:
			res, err = h.alipayService.Query(order.OrderNo)
			if err != nil {
				logger.Errorf("error with query order info: %v", err)
				continue
			}
		}

		// 订单已关闭
		if res.Closed() {
			h.DB.Model(&model.Order{}).Where("id", order.Id).Updates(map[string]any{
				"checked": true,
				"status":  types.OrderPaidFailed,
			})
			logger.Errorf("订单已关闭：%v", order)
			continue
		}

		// 订单未支付，不处理，继续轮询
		if !res.Success() {
			continue
		}

		// 订单支付成功
		err = h.paySuccess(res)
		if err != nil {
			logger.Errorf("error with deal order: %v", err)
			continue
		}
	}
	return nil
}

func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	var data struct {
		PayWay  string `json:"pay_way,omitempty"` // 支付方式：支付宝，微信
		Pid     int    `json:"pid,omitempty"`
		Device  string `json:"device,omitempty"`
		Domain  string `json:"domain,omitempty"` // 支付回调域名
		Channel string `json:"channel,omitempty"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	var product model.Product
	err := h.DB.Where("id", data.Pid).First(&product).Error
	if err != nil {
		resp.ERROR(c, "Product not found")
		return
	}

	orderNo, err := h.snowflake.Next(false)
	if err != nil {
		resp.ERROR(c, "error with generate trade no: "+err.Error())
		return
	}
	userId := h.GetLoginUserId(c)
	var user model.User
	err = h.DB.Where("id", userId).First(&user).Error
	if err != nil {
		resp.NotAuth(c)
		return
	}

	amount := product.Price
	var payURL, notifyURL string
	switch data.PayWay {
	case "wxpay":
		logger.Debugf("微信支付，%+v", data)
		data.Channel = payment.PayChannelWX
		// 优先使用微信官方支付
		if h.config.WxPay.Enabled {
			data.Channel = "wxpay"
			notifyURL = fmt.Sprintf("%s/api/payment/notify/wxpay", h.config.WxPay.Domain)
			payURL, err = h.wxpayService.Pay(payment.PayRequest{
				OutTradeNo: orderNo,
				TotalFee:   fmt.Sprintf("%d", int(amount*100)),
				Subject:    product.Name,
				NotifyURL:  notifyURL,
				ClientIP:   c.ClientIP(),
				Device:     data.Device,
				PayWay:     payment.PayWayWX,
			})
			if err != nil {
				resp.ERROR(c, err.Error())
				return
			}
		} else if h.config.Epay.Enabled { // 聚合支付
			logger.Debugf("聚合支付%+v", data)
			data.Channel = payment.PayChannelEpay
			notifyURL = fmt.Sprintf("%s/api/payment/notify/epay", h.config.Epay.Domain)
			params := payment.PayRequest{
				OutTradeNo: orderNo,
				Subject:    product.Name,
				TotalFee:   fmt.Sprintf("%f", amount),
				ClientIP:   c.ClientIP(),
				Device:     data.Device,
				PayWay:     payment.PayWayWX,
				NotifyURL:  notifyURL,
			}

			r, err := h.epayService.Pay(params)
			logger.Debugf("请求支付结果，%+v", r)
			if err != nil {
				resp.ERROR(c, err.Error())
				return
			} else {
				payURL = r
			}
		} else {
			resp.ERROR(c, "系统没有配置可用的支付渠道！")
			return
		}
	case "alipay":
		if h.config.Alipay.Enabled {
			logger.Debugf("支付宝，%+v", data)
			data.Channel = payment.PayChannelAL
			notifyURL = fmt.Sprintf("%s/api/payment/notify/alipay", h.config.Alipay.Domain)
			money := fmt.Sprintf("%.2f", amount)
			payURL, err = h.alipayService.Pay(payment.PayRequest{
				Device:     data.Device,
				OutTradeNo: orderNo,
				Subject:    product.Name,
				TotalFee:   money,
				NotifyURL:  notifyURL,
			})

			if err != nil {
				resp.ERROR(c, "error with generate pay url: "+err.Error())
				return
			}
		} else if h.config.Epay.Enabled { // 聚合支付
			logger.Debugf("聚合支付，%+v", data)
			data.Channel = payment.PayChannelEpay
			notifyURL = fmt.Sprintf("%s/api/payment/notify/epay", h.config.Epay.Domain)
			params := payment.PayRequest{
				OutTradeNo: orderNo,
				Subject:    product.Name,
				TotalFee:   fmt.Sprintf("%f", amount),
				ClientIP:   c.ClientIP(),
				Device:     data.Device,
				PayWay:     data.PayWay,
				NotifyURL:  notifyURL,
			}

			r, err := h.epayService.Pay(params)
			if err != nil {
				resp.ERROR(c, err.Error())
				return
			} else {
				payURL = r
			}
		} else {
			resp.ERROR(c, "系统没有配置可用的支付渠道！")
			return
		}
	default:
		resp.ERROR(c, "不支持的支付渠道")
		return
	}

	// 创建订单
	remark := types.OrderRemark{
		Power: product.Power,
		Name:  product.Name,
		Price: product.Price,
	}
	order := model.Order{
		UserId:   user.Id,
		Username: user.Username,
		OrderNo:  orderNo,
		Subject:  product.Name,
		Amount:   amount,
		Status:   types.OrderNotPaid,
		PayWay:   data.PayWay,
		Channel:  data.Channel,
		Remark:   utils.JsonEncode(remark),
	}
	err = h.DB.Create(&order).Error
	if err != nil {
		resp.ERROR(c, "error with create order: "+err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"pay_url": payURL, "order_no": orderNo})
}

// paySuccess 支付成功处理（事务 + 行锁保证幂等）
func (h *PaymentHandler) paySuccess(info payment.OrderInfo) error {
	return h.DB.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", info.OutTradeNo).First(&order).Error
		if err != nil {
			return fmt.Errorf("fetch order: %w", err)
		}

		if order.Status == types.OrderPaidSuccess {
			return nil
		}

		if !amountsEqual(info.Amount, order.Amount) {
			return fmt.Errorf("amount mismatch: paid=%s, expected=%.2f", info.Amount, order.Amount)
		}

		var remark types.OrderRemark
		if err := utils.JsonDecode(order.Remark, &remark); err != nil {
			return fmt.Errorf("decode order remark: %w", err)
		}

		err = tx.Model(&model.Order{}).
			Where("id = ? AND status <> ?", order.Id, types.OrderPaidSuccess).
			Updates(map[string]any{
				"status":   types.OrderPaidSuccess,
				"trade_no": info.TradeId,
				"pay_time": utils.Str2stamp(info.PayTime),
				"checked":  true,
			}).Error
		if err != nil {
			return fmt.Errorf("update order: %w", err)
		}

		err = h.userService.IncreasePowerTx(tx, order.UserId, remark.Power, model.PowerLog{
			Type:      types.PowerRecharge,
			Model:     order.Subject,
			Remark:    fmt.Sprintf("充值算力，金额：%.2f，订单号：%s", order.Amount, order.OrderNo),
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("increase power: %w", err)
		}

		err = tx.Model(&model.Product{}).Where("id = ?", order.ProductId).
			UpdateColumn("sales", gorm.Expr("sales + ?", 1)).Error
		if err != nil {
			return fmt.Errorf("update product sales: %w", err)
		}

		return nil
	})
}

func amountsEqual(paidAmount string, orderAmount float64) bool {
	paid, err := strconv.ParseFloat(strings.TrimSpace(paidAmount), 64)
	if err != nil {
		return false
	}
	return int64(math.Round(paid*100)) == int64(math.Round(orderAmount*100))
}

// AlipayNotify 支付宝支付回调
func (h *PaymentHandler) AlipayNotify(c *gin.Context) {
	orderInfo, err := h.alipayService.TradeVerify(c.Request)
	if err != nil {
		logger.Errorf("alipay notify verify failed: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	if !orderInfo.Success() {
		c.String(http.StatusOK, "fail")
		return
	}

	err = h.paySuccess(orderInfo)
	if err != nil {
		logger.Error(err)
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// EPayNotify 易支付支付异步回调
func (h *PaymentHandler) EPayNotify(c *gin.Context) {
	var params = make(map[string]string)
	for k := range c.Request.URL.Query() {
		params[k] = c.Query(k)
	}

	logger.Infof("收到易支付订单支付回调：%+v", params)
	// 检查支付状态, 如果未支付，则返回成功
	if params["trade_status"] != "TRADE_SUCCESS" {
		c.String(http.StatusOK, "success")
		return
	}

	sign := h.epayService.Sign(params)
	if sign != c.Query("sign") {
		logger.Errorf("签名验证失败, %s, %s", sign, c.Query("sign"))
		c.String(http.StatusOK, "fail")
		return
	}
	// 查询订单状态
	order, err := h.epayService.Query(params["out_trade_no"])
	if err != nil {
		logger.Error(err)
		c.String(http.StatusOK, "fail")
		return
	}

	err = h.paySuccess(order)
	if err != nil {
		logger.Error(err)
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// WxpayNotify 微信商户支付异步回调
func (h *PaymentHandler) WxpayNotify(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.String(http.StatusOK, "fail")
		return
	}

	orderInfo, err := h.wxpayService.TradeVerify(c.Request)
	logger.Infof("收到微信商号订单支付回调：%+v", orderInfo)
	if err != nil {
		logger.Errorf("订单校验失败：%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL"})
		return
	}

	err = h.paySuccess(orderInfo)
	if err != nil {
		logger.Error(err)
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}
