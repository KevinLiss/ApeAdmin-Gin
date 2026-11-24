package core

import (
	"path/filepath"
	"testing"

	"apeadmin-gin/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func smokeDBConfig(dir, name string, autoMigrate bool) *config.DatabaseConfig {
	return &config.DatabaseConfig{
		Type:         "sqlite",
		DBName:       filepath.Join(dir, name),
		MaxOpenConns: 2,
		MaxIdleConns: 1,
		LogLevel:     "silent",
		AutoMigrate:  autoMigrate,
	}
}

// 生产路径（auto_migrate=false）必须由内置迁移建出新表。
func TestInitDBWithAutoMigrateDisabledRunsBuiltinMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "migdb.db")

	// 模拟存量库：已有 sys_user（全量建表早已完成）
	seed, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if err := seed.Exec("CREATE TABLE sys_user (id integer PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create sys_user: %v", err)
	}
	CloseDB(seed)

	db, err := InitDB(smokeDBConfig(dir, "migdb", false), nil)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		CloseDB(db)
		ResetTest()
	})
	SetDB(db)

	for _, table := range []string{"sys_chat_run", "sys_oauth_client", "sys_oauth_code", "sys_oauth_refresh_token"} {
		if !tableExists(t, db, table) {
			t.Fatalf("auto_migrate=false 时未创建表 %s", table)
		}
	}
	// 二次启动必须幂等
	if err := RunVersionedMigrations(db, "sqlite"); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
}

// 空库 + auto_migrate=false 必须明确报错（而不是起服务后满屏 500）
func TestInitDBWithEmptyDatabaseAndAutoMigrateDisabledFailsFast(t *testing.T) {
	dir := t.TempDir()
	db, err := InitDB(smokeDBConfig(dir, "emptydb", false), nil)
	if err == nil {
		if db != nil {
			CloseDB(db)
		}
		t.Fatal("空库 + auto_migrate=false 应返回明确错误")
	}
	ResetTest()
}
