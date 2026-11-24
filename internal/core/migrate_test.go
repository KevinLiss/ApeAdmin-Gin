package core

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openMigrateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func tableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return n > 0
}

func TestRunVersionedMigrationsCreatesNewTables(t *testing.T) {
	db := openMigrateTestDB(t)

	if err := RunVersionedMigrations(db, "sqlite"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, table := range []string{"sys_chat_run", "sys_oauth_client", "sys_oauth_code", "sys_oauth_refresh_token", "schema_migration"} {
		if !tableExists(t, db, table) {
			t.Fatalf("表 %s 未创建", table)
		}
	}

	var applied int64
	db.Raw("SELECT count(*) FROM schema_migration").Scan(&applied)
	if applied != 1 {
		t.Fatalf("applied versions = %d, want 1", applied)
	}
}

func TestRunVersionedMigrationsIsIdempotent(t *testing.T) {
	db := openMigrateTestDB(t)

	if err := RunVersionedMigrations(db, "sqlite"); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := RunVersionedMigrations(db, "sqlite"); err != nil {
		t.Fatalf("second migrate must be no-op: %v", err)
	}
	var applied int64
	db.Raw("SELECT count(*) FROM schema_migration").Scan(&applied)
	if applied != 1 {
		t.Fatalf("applied versions = %d after rerun, want 1", applied)
	}

	pending, err := PendingMigrationVersions(db, "sqlite")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %v, want none", pending)
	}
}

func TestPendingMigrationVersionsBeforeApply(t *testing.T) {
	db := openMigrateTestDB(t)
	if err := RunVersionedMigrations(db, "sqlite"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 清空记录 → 视为未应用
	if err := db.Exec("DELETE FROM schema_migration").Error; err != nil {
		t.Fatalf("reset: %v", err)
	}
	pending, err := PendingMigrationVersions(db, "sqlite")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 || pending[0] != "0001_chat_run_oauth" {
		t.Fatalf("pending = %v", pending)
	}
}