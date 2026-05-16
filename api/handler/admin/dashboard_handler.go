package admin

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"geekai/core"
	"geekai/core/types"
	"geekai/handler"
	"geekai/store/model"
	"geekai/utils/resp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	handler.BaseHandler
}

func NewDashboardHandler(app *core.AppServer, db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{BaseHandler: handler.BaseHandler{App: app, DB: db}}
}

// RegisterRoutes 注册路由
func (h *DashboardHandler) RegisterRoutes() {
	group := h.App.Engine.Group("/api/admin/dashboard/")
	group.GET("stats", h.Stats)
}

// statsVo 增加 recentOrders、recentUsers 字段
// 最近订单
type OrderBrief struct {
	OrderNo   string    `json:"order_no"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

// 最近用户
type UserBrief struct {
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	LastActive time.Time `json:"last_active"`
}

type statsVo struct {
	Users        int64                         `json:"users"`
	Tokens       int                           `json:"tokens"`
	Income       float64                       `json:"income"`
	Chart        map[string]map[string]float64 `json:"chart"`
	TodayUsers   int64                         `json:"todayUsers"`
	TodayTokens  int                           `json:"todayTokens"`
	TodayIncome  float64                       `json:"todayIncome"`
	TodayOrders  int64                         `json:"todayOrders"`
	Orders       int64                         `json:"orders"`
	RecentOrders []OrderBrief                  `json:"recentOrders"`
	RecentUsers  []UserBrief                   `json:"recentUsers"`
}

func (h *DashboardHandler) Stats(c *gin.Context) {
	stats := statsVo{}
	now := time.Now()
	zeroTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 总用户数
	h.DB.Model(&model.User{}).Count(&stats.Users)

	// 今日新增用户
	h.DB.Model(&model.User{}).Where("created_at > ?", zeroTime).Count(&stats.TodayUsers)

	// 总算力消耗
	var powerLogs []model.PowerLog
	h.DB.Where("mark = ?", types.PowerSub).Find(&powerLogs)
	for _, item := range powerLogs {
		stats.Tokens += item.Amount
	}

	// 今日算力消耗
	var todayPowerLogs []model.PowerLog
	h.DB.Where("mark = ?", types.PowerSub).Where("created_at > ?", zeroTime).Find(&todayPowerLogs)
	for _, item := range todayPowerLogs {
		stats.TodayTokens += item.Amount
	}

	// 总收入
	var allOrders []model.Order
	h.DB.Where("status = ?", types.OrderPaidSuccess).Find(&allOrders)
	for _, item := range allOrders {
		stats.Income += item.Amount
	}

	// 今日收入
	var todayOrders []model.Order
	h.DB.Where("status = ?", types.OrderPaidSuccess).Where("created_at > ?", zeroTime).Find(&todayOrders)
	for _, item := range todayOrders {
		stats.TodayIncome += item.Amount
	}

	// 订单总数
	h.DB.Model(&model.Order{}).Where("status = ?", types.OrderPaidSuccess).Count(&stats.Orders)

	// 今日订单数
	h.DB.Model(&model.Order{}).Where("status = ?", types.OrderPaidSuccess).Where("created_at > ?", zeroTime).Count(&stats.TodayOrders)

	// recentOrders: 最近10条已支付订单
	var orderList []model.Order
	h.DB.Model(&model.Order{}).Where("status = ?", types.OrderPaidSuccess).Order("created_at desc").Limit(10).Find(&orderList)
	for _, o := range orderList {
		stats.RecentOrders = append(stats.RecentOrders, OrderBrief{
			OrderNo:   o.OrderNo,
			Amount:    o.Amount,
			CreatedAt: o.CreatedAt,
		})
	}
	// recentUsers: 最近10个注册用户
	var userList []model.User
	h.DB.Model(&model.User{}).Order("created_at desc").Limit(10).Find(&userList)
	for _, u := range userList {
		lastActive := u.UpdatedAt
		if lastActive.IsZero() {
			lastActive = u.CreatedAt
		}
		stats.RecentUsers = append(stats.RecentUsers, UserBrief{
			Nickname:   u.Nickname,
			Avatar:     u.Avatar,
			LastActive: lastActive,
		})
	}

	// 统计7天的订单的图表
	startDate := now.Add(-7 * 24 * time.Hour).Format("2006-01-02")
	var statsChart = make(map[string]map[string]float64)
	//// 初始化
	var userStatistic, historyMessagesStatistic, incomeStatistic = make(map[string]float64), make(map[string]float64), make(map[string]float64)
	for i := 0; i < 7; i++ {
		var initTime = time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		userStatistic[initTime] = float64(0)
		historyMessagesStatistic[initTime] = float64(0)
		incomeStatistic[initTime] = float64(0)
	}

	// 统计用户7天增加的曲线
	var users []model.User
	err := h.DB.Model(&model.User{}).Where("created_at > ?", startDate).Find(&users).Error
	if err == nil {
		for _, item := range users {
			userStatistic[item.CreatedAt.Format("2006-01-02")] += 1
		}
	}

	// 统计7天算力消耗
	var chartPowerLogs []model.PowerLog
	err = h.DB.Where("mark = ?", types.PowerSub).Where("created_at > ?", startDate).Find(&chartPowerLogs).Error
	if err == nil {
		for _, item := range chartPowerLogs {
			historyMessagesStatistic[item.CreatedAt.Format("2006-01-02")] += float64(item.Amount)
		}
	}

	// 统计最近7天的订单
	var orders []model.Order
	err = h.DB.Where("status = ?", types.OrderPaidSuccess).Where("created_at > ?", startDate).Find(&orders).Error
	if err == nil {
		for _, item := range orders {
			incomeStatistic[item.CreatedAt.Format("2006-01-02")], _ = decimal.NewFromFloat(incomeStatistic[item.CreatedAt.Format("2006-01-02")]).Add(decimal.NewFromFloat(item.Amount)).Float64()
		}
	}

	statsChart["users"] = userStatistic
	statsChart["historyMessage"] = historyMessagesStatistic
	statsChart["orders"] = incomeStatistic

	stats.Chart = statsChart

	resp.SUCCESS(c, stats)
}
