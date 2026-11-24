package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"apeadmin-gin/internal/pkg/utils"
)

func openSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func writeSeedFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seed.sql")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return path
}

// 分号出现在字符串内不能被当作语句分隔符；注释需先被剥离再校验
func TestSplitSeedStatementsRespectsQuotesAndComments(t *testing.T) {
	sqlText := "INSERT INTO t (msg) VALUES ('a;b'); -- 行注释 DROP TABLE t;\n" +
		"/* 块注释; DELETE FROM t */\nCREATE TABLE x (id INTEGER);"
	stmts, err := utils.SplitSQLStatements(sqlText)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("stmts = %d (%q), want 2", len(stmts), stmts)
	}
	for _, s := range stmts {
		if err := validateSeedStatement(s); err != nil {
			t.Fatalf("合法语句被误拒: %q: %v", s, err)
		}
	}
	if stmts[0] != "INSERT INTO t (msg) VALUES ('a;b')" {
		t.Fatalf("stmts[0] = %q", stmts[0])
	}
}

func TestValidateSeedStatementRejectsDangerousSQL(t *testing.T) {
	bad := []string{
		"DROP TABLE sys_user",
		"DELETE FROM sys_user",
		"UPDATE sys_user SET password='x'",
		"ALTER TABLE sys_user ADD COLUMN x INT",
		"TRUNCATE TABLE sys_user",
		"INSERTX INTO t VALUES (1)",
		"REPLACE INTO t VALUES (1)",
	}
	for _, stmt := range bad {
		if err := validateSeedStatement(stmt); err == nil {
			t.Fatalf("危险语句未被拦截: %q", stmt)
		}
	}

	good := []string{
		"INSERT INTO t VALUES (1)",
		"insert into t values (1)",
		"CREATE TABLE t (id INTEGER)",
		"CREATE INDEX idx_t ON t (id)",
	}
	for _, stmt := range good {
		if err := validateSeedStatement(stmt); err != nil {
			t.Fatalf("合法语句被误拒: %q: %v", stmt, err)
		}
	}
}

// 注释包裹的 DROP 必须在执行前被拒绝（防 -- 注释旁路）
func TestInjectSeedSQLRejectsCommentObfuscatedDrop(t *testing.T) {
	db := openSeedTestDB(t)
	if err := db.Exec("CREATE TABLE guarded (id INTEGER)").Error; err != nil {
		t.Fatalf("prepare table: %v", err)
	}
	path := writeSeedFile(t, "-- 正常注释\nDROP TABLE guarded;")

	if err := injectSeedSQL(db, path); err == nil {
		t.Fatal("DROP 语句必须被拒绝")
	}
	var count int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='guarded'").Scan(&count)
	if count != 1 {
		t.Fatal("被拒绝的 seed.sql 不得改变数据库结构")
	}
}

func TestInjectSeedSQLExtractsAndRunsAllowedStatements(t *testing.T) {
	db := openSeedTestDB(t)
	path := writeSeedFile(t, `
-- 插件预置数据
CREATE TABLE if not exists seed_demo (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO seed_demo (id, name) VALUES (1, 'a;b');
INSERT INTO seed_demo (id, name) VALUES (2, 'c');
`)

	if err := injectSeedSQL(db, path); err != nil {
		t.Fatalf("inject: %v", err)
	}
	var rows []struct {
		ID   int
		Name string
	}
	if err := db.Raw("SELECT id, name FROM seed_demo ORDER BY id").Scan(&rows).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 2 || rows[0].Name != "a;b" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestSplitSeedStatementsRejectsUnclosedQuote(t *testing.T) {
	if _, err := utils.SplitSQLStatements("INSERT INTO t VALUES ('oops);"); err == nil {
		t.Fatal("未闭合引号必须报错")
	}
}
