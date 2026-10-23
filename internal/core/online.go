package core

import (
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// 在线会话注册表
//
// 用途：在线用户列表 + 强制下线。
// JWT 无状态架构下无法从 token 反查在线用户，因此在 JWTAuth 中间件里
// 对每次通过认证的请求登记会话（滑动窗口：15 分钟无活动视为离线），
// 踢下线通过 TokenVersion +1 实现（该用户全部 access/refresh token 立即失效）。
//
// 内存实现（单实例）；多实例需 Redis，接口留空未来可替换。
// ────────────────────────────────────────────────────────────────────────────

// OnlineSession 在线会话记录
type OnlineSession struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	LastSeen  time.Time `json:"last_seen"`
}

// sessionKey 会话主键：userID + jti（同一用户多端登录各算一条）
type sessionKey struct {
	UserID uint
	JTI    string
}

var (
	onlineMu    sync.RWMutex
	onlineMap   = make(map[sessionKey]*OnlineSession)
	// 活跃窗口：超过该时长无请求即视为离线
	onlineActiveWindow = 15 * time.Minute
)

// TrackOnline 登记会话活跃（JWTAuth 每次认证通过后调用）
func TrackOnline(userID uint, username, jti, ip, userAgent string) {
	k := sessionKey{UserID: userID, JTI: jti}
	onlineMu.Lock()
	if s, ok := onlineMap[k]; ok {
		s.IP = ip
		s.UserAgent = userAgent
		s.LastSeen = time.Now()
	} else {
		onlineMap[k] = &OnlineSession{
			UserID:    userID,
			Username:  username,
			IP:        ip,
			UserAgent: userAgent,
			LastSeen:  time.Now(),
		}
	}
	onlineMu.Unlock()
}

// RemoveOnline 会话登出时移除（登出接口调用）
func RemoveOnline(userID uint, jti string) {
	onlineMu.Lock()
	delete(onlineMap, sessionKey{UserID: userID, JTI: jti})
	onlineMu.Unlock()
}

// ListOnline 当前在线用户（活跃窗口内的会话），按最近活跃排序
func ListOnline() []OnlineSession {
	cutoff := time.Now().Add(-onlineActiveWindow)
	onlineMu.RLock()
	result := make([]OnlineSession, 0, len(onlineMap))
	for _, s := range onlineMap {
		if s.LastSeen.After(cutoff) {
			result = append(result, *s)
		}
	}
	onlineMu.RUnlock()
	// 按最近活跃倒序
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].LastSeen.After(result[j-1].LastSeen); j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

// cleanupOnlineSweep 定期回收离线会话条目（防内存增长）
func init() {
	go func() {
		for range time.Tick(10 * time.Minute) {
			cutoff := time.Now().Add(-onlineActiveWindow)
			onlineMu.Lock()
			for k, s := range onlineMap {
				if s.LastSeen.Before(cutoff) {
					delete(onlineMap, k)
				}
			}
			onlineMu.Unlock()
		}
	}()
}
