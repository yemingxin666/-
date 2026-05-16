package store

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"fmt"
	"geekai/core/types"
	logger2 "geekai/logger"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var log = logger2.GetLogger()

func NewGormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "puningai_", // 设置表前缀
			SingularTable: false,     // 使用单数表名形式
		},
	}
}

func NewMysql(config *gorm.Config, appConfig *types.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(appConfig.MysqlDns), config)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(32)
	sqlDB.SetMaxOpenConns(512)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Info("开始重命名数据表...")

	// 重命名数据表
	tableRenames := map[string]string{
		"geekai_admin_users":     "puningai_admin_users",
		"geekai_users":           "puningai_users",
		"geekai_orders":          "puningai_orders",
		"geekai_products":        "puningai_products",
		"geekai_configs":         "puningai_configs",
		"geekai_files":           "puningai_files",
		"geekai_menus":           "puningai_menus",
		"geekai_invite_codes":    "puningai_invite_codes",
		"geekai_invite_logs":     "puningai_invite_logs",
		"geekai_redeems":         "puningai_redeems",
		"geekai_power_logs":      "puningai_power_logs",
		"geekai_user_login_logs": "puningai_user_login_logs",
	}

	// 执行重命名操作
	for oldTableName, newTableName := range tableRenames {
		// 检查新表是否已存在
		if !db.Migrator().HasTable(newTableName) {
			err := db.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldTableName, newTableName)).Error
			if err != nil {
				log.Errorf("重命名数据表 %s 到 %s 失败: %v", oldTableName, newTableName, err)
			} else {
				log.Infof("成功重命名数据表: %s -> %s", oldTableName, newTableName)
			}
		}
	}
	return db, nil
}
