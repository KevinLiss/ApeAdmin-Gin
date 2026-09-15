package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gin-apeadmin/internal/dal"
	"gin-apeadmin/internal/model"

	"gorm.io/gorm"
)

// LoadL2Plugin 加载 L2 声明式清单插件
// 流程：校验 ZIP → 解压到隔离目录 → 注入 DB → 原子移动到正式目录
func LoadL2Plugin(zipPath string, cfg LoaderConfig) (*Manifest, error) {
	// 1. ZIP 安全校验
	manifest, entries, err := ValidateZip(zipPath, cfg.ZipGuard)
	if err != nil {
		return nil, err
	}

	// 2. 检查是否已存在同名插件
	existing, err := dal.GetPluginByName(manifest.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("插件 %s 已存在，请先卸载旧版本", manifest.Name)
	}

	// 3. 解压到隔离目录
	stagingDir := filepath.Join(cfg.UploadDir, "_staging", manifest.Name+"-"+sha8(zipPath))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("创建隔离目录失败: %w", err)
	}
	if err := extractZip(zipPath, stagingDir); err != nil {
		os.RemoveAll(stagingDir)
		return nil, fmt.Errorf("解压失败: %w", err)
	}

	// 4. 注入 DB（单事务）
	db := dal.GetDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		// sys_plugin
		pluginRecord := model.SysPlugin{
			Name:        manifest.Name,
			DisplayName: manifest.DisplayName,
			Description: manifest.Description,
			Version:     manifest.Version,
			Author:      manifest.Author,
			Enabled:     false,
			ModulePath:  stagingDir,
		}
		if err := tx.Create(&pluginRecord).Error; err != nil {
			return fmt.Errorf("写入插件记录失败: %w", err)
		}

		// 注入菜单（如果 ZIP 包含 menu.json）
		if containsFile(entries, "menu.json") {
			menuPath := filepath.Join(stagingDir, "menu.json")
			if err := injectMenus(tx, menuPath); err != nil {
				return fmt.Errorf("注入菜单失败: %w", err)
			}
		}

		// 注入权限（菜单的 permission 字段自动注册）
		// 注入预置数据（如果 ZIP 包含 seed.sql）
		if containsFile(entries, "seed.sql") {
			seedPath := filepath.Join(stagingDir, "seed.sql")
			if err := injectSeedSQL(tx, seedPath); err != nil {
				return fmt.Errorf("预置数据失败: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		os.RemoveAll(stagingDir)
		return nil, err
	}

	// 5. 原子移动到正式目录
	finalDir := filepath.Join(cfg.UploadDir, manifest.Name+"-"+manifest.Version)
	_ = os.RemoveAll(finalDir) // 如果旧目录存在先删除
	if err := os.Rename(stagingDir, finalDir); err != nil {
		// rename 失败不回滚 DB（插件记录仍在，目录可手动修复）
		return manifest, nil
	}

	return manifest, nil
}

// LoaderConfig 加载器配置
type LoaderConfig struct {
	UploadDir string
	ZipGuard  ZipGuardConfig
}

// extractZip 解压 ZIP 到目标目录
func extractZip(zipPath, destDir string) error {
	r, err := zipOpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		destPath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(destPath), 0755)
		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// injectMenus 从 menu.json 注入菜单
func injectMenus(tx *gorm.DB, menuPath string) error {
	data, err := os.ReadFile(menuPath)
	if err != nil {
		return err
	}
	var menus []model.SysMenu
	if err := jsonUnmarshal(data, &menus); err != nil {
		return err
	}
	for i := range menus {
		if menus[i].Status == 0 {
			menus[i].Status = 1
		}
		if menus[i].Visible == 0 {
			menus[i].Visible = 1
		}
		if err := tx.Create(&menus[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// injectSeedSQL 执行预置 SQL
func injectSeedSQL(tx *gorm.DB, seedPath string) error {
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return tx.Exec(string(data)).Error
}

// containsFile 检查文件列表中是否包含指定文件
func containsFile(entries []string, name string) bool {
	for _, e := range entries {
		if e == name {
			return true
		}
	}
	return false
}

// sha8 计算文件 SHA256 前 8 位
func sha8(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}
