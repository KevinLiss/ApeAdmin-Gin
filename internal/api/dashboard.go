package api

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
)

// DashboardHandler 仪表盘 Handler
type DashboardHandler struct{}

// 进程启动时间（运行时长）
var processStartTime = time.Now()

// 网络速率模块级缓存
var (
	lastNetMu    sync.Mutex
	lastNetStats *netIOCounters
)

type netIOCounters struct {
	timestamp time.Time
	sent      uint64
	recv      uint64
}

// System 系统监控数据（GET /dashboard/system，前端 5 秒轮询）
func (h *DashboardHandler) System(c *gin.Context) {
	db := core.GetDB()
	userID := c.GetUint("user_id")

	var user model.SysUser
	db.First(&user, userID)

	// CPU
	coresLogical, _ := cpu.Counts(true)
	coresPhysical, _ := cpu.Counts(false)
	cpuPercent := 0.0
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		cpuPercent = percents[0]
	}
	loadAvg := []float64{0, 0, 0}
	if avg, err := load.Avg(); err == nil {
		loadAvg = []float64{avg.Load1, avg.Load5, avg.Load15}
	}

	// 内存
	memInfo := map[string]interface{}{}
	if vm, err := mem.VirtualMemory(); err == nil {
		swapTotal, swapUsed := uint64(0), uint64(0)
		if sw, err := mem.SwapMemory(); err == nil {
			swapTotal, swapUsed = sw.Total, sw.Used
		}
		memInfo = map[string]interface{}{
			"total": vm.Total, "used": vm.Used, "available": vm.Available,
			"percent": round1(vm.UsedPercent),
			"swap_total": swapTotal, "swap_used": swapUsed, "swap_percent": 0.0,
		}
	}

	// 磁盘（当前目录所在盘）
	diskInfo := map[string]interface{}{}
	if wd, err := os.Getwd(); err == nil {
		if du, err := disk.Usage(wd); err == nil {
			diskInfo = map[string]interface{}{
				"total": du.Total, "used": du.Used, "free": du.Free,
				"percent": round1(du.UsedPercent),
				"read_count": 0, "write_count": 0,
			}
		}
	}

	// 网络（速率 = 两次请求差值）
	networkInfo := collectNetwork()

	// 在线用户（近 24 小时登录过）
	var users []model.SysUser
	db.Where("last_login_at > ?", time.Now().Add(-24*time.Hour)).
		Order("last_login_at DESC").Limit(20).Find(&users)
	onlineUsers := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		lastLogin := ""
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format("2006-01-02 15:04:05")
		}
		onlineUsers = append(onlineUsers, map[string]interface{}{
			"id": u.ID, "username": u.Username, "nickname": u.Nickname,
			"last_login_at": lastLogin, "last_login_ip": u.LastLoginIP,
		})
	}

	// MCP 工具
	tools := make([]map[string]interface{}, 0)
	if manager := core.GetMCPManager(); manager != nil {
		for _, t := range manager.ListTools() {
			tools = append(tools, map[string]interface{}{
				"name": t.Name, "description": t.Description,
				"required_permissions": t.RequiredPermissions,
			})
		}
	}

	// 插件（前 8 条）
	var plugins []model.SysPlugin
	db.Limit(8).Find(&plugins)
	pluginRows := make([]map[string]interface{}, 0, len(plugins))
	for _, p := range plugins {
		pluginRows = append(pluginRows, map[string]interface{}{
			"id": p.ID, "name": p.Name, "display_name": p.DisplayName,
			"version": p.Version, "enabled": p.Enabled,
			"created_at": p.CreatedAt.Format("2006-01-02"),
		})
	}

	// 系统信息
	hostname, _ := os.Hostname()
	osName, uptime := "", time.Duration(0)
	if hi, err := host.Info(); err == nil {
		osName = hi.Platform + " " + hi.PlatformVersion
		uptime = time.Duration(hi.Uptime) * time.Second
	}
	processCount := 0
	if pids, err := process.Pids(); err == nil {
		processCount = len(pids)
	}

	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"user": map[string]interface{}{"username": user.Username, "nickname": user.Nickname},
		"cpu": map[string]interface{}{
			"percent":        round1(cpuPercent),
			"cores_logical":  coresLogical,
			"cores_physical": coresPhysical,
			"load_avg":       loadAvg,
		},
		"memory":  memInfo,
		"disk":    diskInfo,
		"network": networkInfo,
		"system": map[string]interface{}{
			"hostname":       hostname,
			"os":             osName,
			"uptime_seconds": int64(uptime.Seconds()),
			"process_count":  processCount,
		},
		"mcp_tools":    tools,
		"online_users": onlineUsers,
		"plugins":      pluginRows,
	}))
}

// Stats 统计数据（GET /dashboard/stats）
func (h *DashboardHandler) Stats(c *gin.Context) {
	db := core.GetDB()

	var userTotal, roleTotal, menuTotal, deptTotal, pluginTotal, pluginEnabled int64
	db.Model(&model.SysUser{}).Count(&userTotal)
	db.Model(&model.SysRole{}).Count(&roleTotal)
	db.Model(&model.SysMenu{}).Count(&menuTotal)
	db.Model(&model.SysDept{}).Count(&deptTotal)
	db.Model(&model.SysPlugin{}).Count(&pluginTotal)
	db.Model(&model.SysPlugin{}).Where("enabled = ?", true).Count(&pluginEnabled)

	// 审计统计
	var auditTotal, auditToday int64
	db.Model(&model.SysMcpAuditLog{}).Count(&auditTotal)
	db.Model(&model.SysMcpAuditLog{}).
		Where("created_at >= ?", time.Now().Format("2006-01-02")).Count(&auditToday)

	type auditRow struct {
		ID         uint      `json:"id"`
		ActionType string    `json:"action_type"`
		TargetName string    `json:"target_name"`
		Username   string    `json:"username"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
	}
	var recent []auditRow
	db.Model(&model.SysMcpAuditLog{}).Order("id DESC").Limit(5).Find(&recent)
	recentItems := make([]map[string]interface{}, 0, len(recent))
	for _, r := range recent {
		recentItems = append(recentItems, map[string]interface{}{
			"id": r.ID, "action_type": r.ActionType, "target_name": r.TargetName,
			"username": r.Username, "status": r.Status,
			"created_at": r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// 14 天趋势
	dates := make([]string, 14)
	counts := make([]int64, 14)
	for i := 13; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i)
		dates[13-i] = day.Format("01-02")
		var cnt int64
		db.Model(&model.SysMcpAuditLog{}).
			Where("created_at >= ? AND created_at < ?", day.Format("2006-01-02"), day.AddDate(0, 0, 1).Format("2006-01-02")).
			Count(&cnt)
		counts[13-i] = cnt
	}

	// 插件
	var plugins []model.SysPlugin
	db.Find(&plugins)
	pluginRows := make([]map[string]interface{}, 0, len(plugins))
	for _, p := range plugins {
		pluginRows = append(pluginRows, map[string]interface{}{
			"id": p.ID, "name": p.Name, "display_name": p.DisplayName,
			"version": p.Version, "enabled": p.Enabled,
			"created_at": p.CreatedAt.Format("2006-01-02"),
		})
	}

	userID := c.GetUint("user_id")
	var user model.SysUser
	db.First(&user, userID)

	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"user": map[string]interface{}{"username": user.Username, "nickname": user.Nickname},
		"stats": map[string]interface{}{
			"user_total": userTotal, "role_total": roleTotal, "menu_total": menuTotal,
			"dept_total": deptTotal, "plugin_total": pluginTotal, "plugin_enabled": pluginEnabled,
		},
		"audit":   map[string]interface{}{"total": auditTotal, "today": auditToday, "recent": recentItems},
		"trend":   map[string]interface{}{"dates": dates, "counts": counts},
		"plugins": pluginRows,
	}))
}

// collectNetwork 网络速率（两次调用差值 / 时间）
func collectNetwork() map[string]interface{} {
	lastNetMu.Lock()
	defer lastNetMu.Unlock()

	sent, recv := uint64(0), uint64(0)
	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		sent, recv = counters[0].BytesSent, counters[0].BytesRecv
	}

	now := time.Now()
	result := map[string]interface{}{
		"bytes_sent": sent, "bytes_recv": recv,
		"packets_sent": 0, "packets_recv": 0,
		"sent_rate": 0.0, "recv_rate": 0.0,
	}
	if lastNetStats != nil {
		elapsed := now.Sub(lastNetStats.timestamp).Seconds()
		if elapsed > 0.5 {
			sRate := 0.0
			rRate := 0.0
			if sent >= lastNetStats.sent {
				sRate = float64(sent-lastNetStats.sent) / elapsed
			}
			if recv >= lastNetStats.recv {
				rRate = float64(recv-lastNetStats.recv) / elapsed
			}
			result["sent_rate"] = round1(sRate)
			result["recv_rate"] = round1(rRate)
		}
	}
	lastNetStats = &netIOCounters{timestamp: now, sent: sent, recv: recv}
	return result
}

// round1 保留 1 位小数
func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

var _ = fmt.Sprintf
var _ = runtime.NumCPU
