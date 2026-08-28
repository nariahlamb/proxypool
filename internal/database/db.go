package database

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/One-Piecs/proxypool/log"

	"github.com/One-Piecs/proxypool/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func connect() (err error) {
	// Default SQLite file path
	dbPath := "data/proxypool.db"

	// Check config override
	if url := config.Config().DatabaseUrl; url != "" {
		dbPath = url
	}
	// Check env override
	if url := os.Getenv("DATABASE_URL"); url != "" {
		dbPath = url
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Warnln("database: failed to create directory %s: %v", dir, err)
	}

	// WAL 模式：读不阻塞写、写不阻塞读（健康检查 upsert / 冻结 / best 保存并发时减少锁冲突）；
	// busy_timeout：写锁等待 5s 而非立即报 database is locked；_pragma 由 glebarez/sqlite 驱动透传
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	}
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err == nil {
		// SQLite 单写者模型：限制并发连接数，避免写放大与锁竞争
		if sqlDB, e := DB.DB(); e == nil {
			sqlDB.SetMaxOpenConns(5)
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetConnMaxLifetime(0)
		}
		log.Infoln("database: successfully connected to sqlite: %s", dbPath)
	} else {
		DB = nil
		log.Warnln("database connection info: %s \n\t\tUse cache to store proxies", err.Error())
	}
	return
}
