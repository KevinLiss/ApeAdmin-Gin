package plugin

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gin-apeadmin/internal/config"
)

// Manifest L2 声明式插件清单
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Type        string `json:"type"` // 必须为 "l2"
}

// ZipGuardConfig ZIP 安全校验配置
type ZipGuardConfig = config.ZipGuardConfig

// ValidateZip ZIP 安全校验五道闸
// 返回解析后的 Manifest 和安全通过后的临时解压路径
func ValidateZip(zipPath string, cfg ZipGuardConfig) (*Manifest, []string, error) {
	// 1. 压缩包大小限制
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取文件: %w", err)
	}
	if info.Size() > int64(cfg.MaxZipSizeMB)*1024*1024 {
		return nil, nil, fmt.Errorf("压缩包超过大小限制 %dMB", cfg.MaxZipSizeMB)
	}

	// 2. 打开 ZIP 并逐条目校验
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("无法打开 ZIP: %w", err)
	}
	defer r.Close()

	var totalUncompressed int64
	var entries []string
	var hasPluginJSON bool

	if len(r.File) > cfg.MaxEntries {
		return nil, nil, fmt.Errorf("ZIP 条目数超过限制 %d", cfg.MaxEntries)
	}

	for _, f := range r.File {
		// 3. 解压总量限制（防 zip bomb）
		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > int64(cfg.MaxDecompressedMB)*1024*1024 {
			return nil, nil, fmt.Errorf("解压后总量超过限制 %dMB", cfg.MaxDecompressedMB)
		}

		// 4. 路径穿越检查
		name := f.Name
		if strings.Contains(name, "..") || filepath.IsAbs(name) || strings.Contains(name, "C:") {
			return nil, nil, fmt.Errorf("路径穿越拒绝: %s", name)
		}
		if cfg.DenySymlinks && (f.Mode()&os.ModeSymlink != 0) {
			return nil, nil, fmt.Errorf("符号链接拒绝: %s", name)
		}

		// 拒绝可执行文件
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".so" || ext == ".dll" || ext == ".exe" || ext == ".bin" {
			return nil, nil, fmt.Errorf("L2 插件不允许可执行文件: %s", name)
		}

		entries = append(entries, name)
		if name == "plugin.json" {
			hasPluginJSON = true
		}
	}

	// 5. 清单校验
	if !hasPluginJSON {
		return nil, nil, errors.New("缺少 plugin.json 清单文件")
	}

	// 读取并解析 plugin.json
	for _, f := range r.File {
		if f.Name == "plugin.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, nil, fmt.Errorf("读取 plugin.json 失败: %w", err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, nil, fmt.Errorf("读取 plugin.json 失败: %w", err)
			}
			manifest, err := parseManifest(data)
			if err != nil {
				return nil, nil, err
			}
			if manifest.Type != "l2" {
				return nil, nil, fmt.Errorf("插件类型必须为 l2，当前: %s", manifest.Type)
			}
			if manifest.Name == "" {
				return nil, nil, errors.New("plugin.json 缺少 name 字段")
			}
			if manifest.Version == "" {
				return nil, nil, errors.New("plugin.json 缺少 version 字段")
			}
			return manifest, entries, nil
		}
	}

	return nil, nil, errors.New("plugin.json 未找到")
}

// parseManifest 解析 plugin.json
func parseManifest(data []byte) (*Manifest, error) {
	// 使用 encoding/json 解析
	m := &Manifest{}
	if err := jsonUnmarshal(data, m); err != nil {
		return nil, fmt.Errorf("解析 plugin.json 失败: %w", err)
	}
	return m, nil
}
