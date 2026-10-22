package api

import (
	"archive/zip"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"runtime"
	"strings"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ────────────────────────────────────────────────────────────────────────────
// 版本更新（Go 版自更新）
//
// 设计：
//  1. GET /system/version 返回当前版本 + 运行时信息（Go 版本/OS/架构/PID）
//  2. POST /system/update 接收 ZIP 部署包（官网「安装下载」发布的 src 包或 deploy 包）：
//     - 校验 zip 结构（必须含 frontend/dist 或 release-package 结构，防误传）
//     - 备份现有 frontend/dist 与二进制到 .backup
//     - 替换前端 dist；按平台替换二进制（旧二进制 rename 为 .old，Windows 下占用文件也可改名）
//     - 触发优雅停机，由外部守护进程（systemd/supervisor）拉起新进程
//  3. 前端轮询 /health（带 PID）确认新进程已恢复
//
// 安全约束：
//  - 仅接受 .zip（与官网发布物一致）
//  - zip 条目路径穿越防护（.. 与绝对路径拒绝）
//  - zip 炸弹防护（解压总大小上限 1GB、文件数上限 5000）
//  - 权限点 system:setting:edit（与系统设置同级管理能力）
// ────────────────────────────────────────────────────────────────────────────

const (
	updateMaxUncompressed = int64(1 << 30) // 1GB 解压上限
	updateMaxEntries      = 5000           // 条目数上限
	updateBackupSuffix    = ".backup"
	updateOldBinarySuffix = ".old"
)

// VersionHandler 系统版本与自更新
type VersionHandler struct{}

// GetVersion GET /system/version
func (h *VersionHandler) GetVersion(c *gin.Context) {
	cfg := core.GetConfig()
	version := "unknown"
	appName := "ApeAdmin-Gin"
	if cfg != nil {
		if cfg.App.Version != "" {
			version = cfg.App.Version
		}
		if cfg.App.Name != "" {
			appName = cfg.App.Name
		}
	}
	c.JSON(http.StatusOK, response.Success(gin.H{
		"current_version": version,
		"app_name":        appName,
		"runtime":         "go",
		"go_version":      runtime.Version(),
		"os":              runtime.GOOS,
		"arch":            runtime.GOARCH,
		"pid":             os.Getpid(),
	}))
}

// Update POST /system/update —— 上传 ZIP 部署包并执行原地替换
func (h *VersionHandler) Update(c *gin.Context) {
	cfg := core.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.internal_error", nil))
		return
	}

	// 1. 接收文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.file_missing", nil))
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "version.zip_only", nil))
		return
	}
	if file.Size > 200*1024*1024 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "version.too_large", nil))
		return
	}

	// 2. 落盘到临时文件
	tmpDir := filepath.Join(os.TempDir(), "apeadmin-update")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "version.prepare_fail", nil))
		return
	}
	tmpZip := filepath.Join(tmpDir, fmt.Sprintf("pkg-%d.zip", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(file, tmpZip); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "version.save_fail", nil))
		return
	}
	defer os.Remove(tmpZip)

	// 3. 安全解包（穿越/炸弹防护），解到临时目录
	stageDir := filepath.Join(tmpDir, fmt.Sprintf("stage-%d", time.Now().UnixNano()))
	defer os.RemoveAll(stageDir)
	if err := safeUnzip(tmpZip, stageDir); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, err.Error()))
		return
	}

	// 4. 定位包内的有效载荷（deploy 包直含 frontend/dist；src 包内嵌 release-package/）
	spaDir, binDir, newVersion, err := locatePayload(stageDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, err.Error()))
		return
	}

	// 5. 备份现有 dist + 二进制
	workDir := workingDir()
	bakDir := filepath.Join(workDir, "uploads", "version-backup", time.Now().Format("20060102-150405"))
	exePath, exeErr := os.Executable()
	if err := doBackup(workDir, bakDir, exePath, exeErr == nil); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(400, "备份失败: "+err.Error()))
		return
	}

	// 6. 替换：前端 dist 先行（不影响运行中进程），二进制最后替换
	if err := replaceDir(spaDir, cfg.App.SPADir); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(400, "更新前端失败: "+err.Error()))
		return
	}
	oldPid := os.Getpid()
	if binDir != "" {
		if err := replaceBinary(binDir, exePath); err != nil {
			// 二进制替换失败不回滚 dist（前端已是新版也无害），提示手动处理
			c.JSON(http.StatusInternalServerError, response.Error(400, "更新二进制失败: "+err.Error()))
			return
		}
	}
	// 同步本地 config.yaml 的 version 行到新版本号（仅 patch version: 一行，
	// 数据库/JWT 等用户自定义配置不动），使重启后 /system/version 显示新版本
	if newVersion != "" {
		_ = patchLocalConfigVersion(filepath.Join(workDir, "configs", "config.yaml"), newVersion)
	}

	// 7. 触发优雅停机（由守护进程拉起新进程；裸进程部署则需手动重启）
	go func() {
		time.Sleep(500 * time.Millisecond) // 等响应送达客户端
		core.RequestShutdown()
	}()

	c.JSON(http.StatusOK, response.Success(gin.H{
		"message":       response.SuccessCtx(c, "version.update_ok", nil).Msg,
		"old_pid":       oldPid,
		"new_version":   newVersion,
		"backup_dir":    bakDir,
		"binary_update": binDir != "",
	}))
}

// ────────────────────────────────────────────────────────────────────────────
// 内部工具

// workingDir 工作目录（配置相对路径的基准）
func workingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// safeUnzip 带 zip 炸弹与路径穿越防护的解包
func safeUnzip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法读取 ZIP 包: %w", err)
	}
	defer zr.Close()

	if len(zr.File) > updateMaxEntries {
		return fmt.Errorf("ZIP 条目数超过限制 %d", updateMaxEntries)
	}

	var totalUncompressed int64
	for _, f := range zr.File {
		totalUncompressed += int64(f.UncompressedSize64)
	}
	if totalUncompressed > updateMaxUncompressed {
		return fmt.Errorf("解压后总大小超过限制 1GB")
	}

	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if strings.HasPrefix(f.Name, "/") || strings.Contains(name, "..") {
			return fmt.Errorf("检测到非法路径条目: %s", f.Name)
		}
		target := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := copyWithLimit(dst, rc, f.UncompressedSize64); err != nil {
			rc.Close()
			dst.Close()
			return err
		}
		rc.Close()
		dst.Close()
	}
	return nil
}

// copyWithLimit 复制并在实际写入超出声明大小时报错（防炸弹）
func copyWithLimit(dst *os.File, rc interface{ Read([]byte) (int, error) }, declared uint64) (int64, error) {
	buf := make([]byte, 64*1024)
	var written int64
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			written += int64(n)
			if written > int64(declared) || written > updateMaxUncompressed {
				return written, fmt.Errorf("解压大小超出声明，疑似 zip 炸弹")
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return written, werr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				return written, nil
			}
			return written, err
		}
	}
}

// locatePayload 在解包目录中定位有效载荷，返回（新dist目录, 新二进制目录, 新版本号）
// 兼容两种包结构：
//   - deploy 包：直接含 frontend/dist、bin/、configs/
//   - src 包：顶层即源码，内嵌 release-package/frontend/dist 与 release-package/bin/
func locatePayload(stage string) (spaDir string, binDir string, version string, err error) {
	// 探测函数：给定根目录，检查是否含 frontend/dist 或 configs
	probe := func(root string) (spa string, bin string, ver string, ok bool) {
		dist := filepath.Join(root, "frontend", "dist")
		if st, e := os.Stat(dist); e == nil && st.IsDir() {
			spa = dist
		}
		bin = filepath.Join(root, "bin")
		if st, e := os.Stat(bin); e == nil && st.IsDir() {
			bin = bin
		} else {
			bin = ""
		}
		// 版本号从包内 config.yaml 读
		if v, e := readVersionFromConfig(filepath.Join(root, "configs", "config.yaml")); e == nil {
			ver = v
		}
		ok = spa != ""
		return
	}

	// 优先按 deploy 包结构探测（顶层）
	if spa, bin, ver, ok := probe(stage); ok {
		return spa, bin, ver, nil
	}
	// 其次按 src 包结构探测（release-package/ 内嵌）
	rp := filepath.Join(stage, "release-package")
	if spa, bin, ver, ok := probe(rp); ok {
		return spa, bin, ver, nil
	}
	return "", "", "", fmt.Errorf("包内未找到 frontend/dist 目录，请确认上传的是 ApeAdmin-Gin 官方 ZIP 部署包")
}

// readVersionFromConfig 从 config.yaml 提取 version 字段（轻量解析，避免引入 viper 依赖）
func readVersionFromConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version:") {
			v := strings.Trim(strings.TrimPrefix(line, "version:"), ` "'`)
			if v != "" {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("no version")
}

// patchLocalConfigVersion 将本地 config.yaml 的 version: 行替换为新版本号。
// 仅改这一行，保留用户其余全部配置；找不到 version 行时不做任何修改。
func patchLocalConfigVersion(path, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	patched := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "version:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "version: " + strconv.Quote(newVersion)
			patched = true
			break
		}
	}
	if !patched {
		return fmt.Errorf("config 中无 version 行")
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// doBackup 备份现有前端 dist 与可执行文件到备份目录
func doBackup(workDir, bakDir, exePath string, hasExe bool) error {
	if err := os.MkdirAll(bakDir, 0755); err != nil {
		return err
	}
	cfg := core.GetConfig()
	spaDir := cfg.App.SPADir
	if !filepath.IsAbs(spaDir) {
		spaDir = filepath.Join(workDir, spaDir)
	}
	if st, err := os.Stat(spaDir); err == nil && st.IsDir() {
		dst := filepath.Join(bakDir, "frontend-dist")
		if err := copyDir(spaDir, dst); err != nil {
			return err
		}
	}
	if hasExe {
		dst := filepath.Join(bakDir, "binary")
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		if _, err := copyFile(exePath, filepath.Join(dst, filepath.Base(exePath))); err != nil {
			return err
		}
	}
	return nil
}

// replaceDir 用新目录替换旧目录（旧目录 rename 到 .backup-tmp 后删除，新目录 rename 过去）
func replaceDir(srcDir, dstRel string) error {
	dst := dstRel
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(workingDir(), dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	// 旧目录挪走（失败可忽略，rename 覆盖由 os.Rename 目录实现需要目标不存在）
	if _, err := os.Stat(dst); err == nil {
		old := dst + updateBackupSuffix
		_ = os.RemoveAll(old)
		if err := os.Rename(dst, old); err != nil {
			return fmt.Errorf("无法挪走旧目录: %w", err)
		}
		defer os.RemoveAll(old)
	}
	return os.Rename(srcDir, dst)
}

// replaceBinary 按平台选择新二进制替换当前进程文件
// 当前运行的二进制无法直接覆盖（Windows 锁定 / Unix inode 换名后旧进程仍跑旧码），
// 标准做法：旧文件 rename 为 .old，新文件 copy 到原路径
func replaceBinary(binDir, exePath string) error {
	if exePath == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(binDir, "ApeAdmin-Gin_"+runtime.GOOS+"_"+runtime.GOARCH),
		filepath.Join(binDir, "ApeAdmin-Gin_"+runtime.GOOS+"_"+runtime.GOARCH+".exe"),
	}
	var newBin string
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			newBin = p
			break
		}
	}
	if newBin == "" {
		return fmt.Errorf("包内未找到 %s/%s 平台二进制", runtime.GOOS, runtime.GOARCH)
	}

	// 旧二进制挪名（Windows 允许 rename 正在运行的 exe）
	if _, err := os.Stat(exePath); err == nil {
		old := exePath + updateOldBinarySuffix
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return fmt.Errorf("无法挪走旧二进制: %w", err)
		}
	}
	if _, err := copyFile(newBin, exePath); err != nil {
		return fmt.Errorf("写入新二进制失败: %w", err)
	}
	if err := os.Chmod(exePath, 0755); err != nil {
		return err
	}
	return nil
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if _, err := copyFile(path, target); err != nil {
			return err
		}
		return nil
	})
}

// copyFile 复制单个文件（保留可执行权限）
func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return 0, err
	}
	perm := st.Mode().Perm()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return written, werr
			}
			written += int64(n)
		}
		if rerr != nil {
			if rerr.Error() == "EOF" {
				return written, nil
			}
			return written, rerr
		}
	}
}
