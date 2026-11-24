package core

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"apeadmin-gin/internal/pkg/utils"

	"gorm.io/gorm"
)

// 内置版本化迁移脚本（sqlite / mysql 两套方言）。
//
// 背景：config.yaml 注释要求生产 auto_migrate=false 并走 golang-migrate，
// 但仓库此前并没有 migrations 目录 —— 关闭 AutoMigrate 后新表（sys_chat_run、
// sys_oauth_*）永远不会被创建，升级即报错。
// 现在把迁移脚本随二进制 embed，auto_migrate=false 时启动自动执行未应用的版本，
// 升级只需替换二进制；脚本全部幂等（IF NOT EXISTS），可重复执行。
//
//go:embed migrations/sqlite/*.sql migrations/mysql/*.sql
var migrationFiles embed.FS

const migrationVersionTable = "schema_migration"

// RunVersionedMigrations 执行内置迁移中尚未应用的版本。
// 版本记录写入 schema_migration（version 主键 + applied_at），按文件名升序执行。
func RunVersionedMigrations(db *gorm.DB, dbType string) error {
	dir := path.Join("migrations", dbType)
	files, err := fs.Glob(migrationFiles, dir+"/*.sql")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	if len(files) == 0 {
		return nil
	}
	sort.Strings(files)

	if err := db.Exec(fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS `%s` (`version` VARCHAR(128) NOT NULL PRIMARY KEY, `applied_at` DATETIME NULL)",
		migrationVersionTable)).Error; err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	applied := map[string]bool{}
	rows, err := db.Raw(fmt.Sprintf("SELECT `version` FROM `%s`", migrationVersionTable)).Rows()
	if err != nil {
		return fmt.Errorf("查询已应用迁移失败: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, file := range files {
		version := strings.TrimSuffix(path.Base(file), ".sql")
		if applied[version] {
			continue
		}
		raw, err := migrationFiles.ReadFile(file)
		if err != nil {
			return err
		}
		stmts, err := utils.SplitSQLStatements(string(raw))
		if err != nil {
			return fmt.Errorf("迁移 %s 解析失败: %w", version, err)
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range stmts {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return tx.Exec(fmt.Sprintf("INSERT INTO `%s` (`version`, `applied_at`) VALUES (?, ?)", migrationVersionTable),
				version, time.Now()).Error
		}); err != nil {
			return fmt.Errorf("迁移 %s 执行失败: %w", version, err)
		}
		log.Printf("[MIGRATE] applied %s", version)
	}
	return nil
}

// PendingMigrationVersions 返回尚未应用的版本号（供启动日志/自检提示）
func PendingMigrationVersions(db *gorm.DB, dbType string) ([]string, error) {
	dir := path.Join("migrations", dbType)
	files, err := fs.Glob(migrationFiles, dir+"/*.sql")
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	rows, err := db.Raw(fmt.Sprintf("SELECT `version` FROM `%s`", migrationVersionTable)).Rows()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		applied[v] = true
	}
	rows.Close()

	var pending []string
	sort.Strings(files)
	for _, f := range files {
		v := strings.TrimSuffix(path.Base(f), ".sql")
		if !applied[v] {
			pending = append(pending, v)
		}
	}
	return pending, nil
}
