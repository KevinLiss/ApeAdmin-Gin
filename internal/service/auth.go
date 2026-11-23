package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/schema"
)

// ────────────────────────────────────────────────────────────────────────────
// 权限缓存
//
// 背景：RequirePermission 中间件原先每个请求触发 2+N 次查库
// （用户+角色 Preload、每角色一条 Raw SQL），是 API 全链路最大热点。
//
// 实现：
//   - 进程内 TTL 缓存（默认 30s），userID → 权限集合
//   - 角色变更（AssignRoleMenus / 角色启停 / 角色删除）时主动失效
//   - 单机部署；多实例时 TTL 兜底一致性（30s 内其他实例仍用旧权限，可接受）
// ────────────────────────────────────────────────────────────────────────────

const permCacheTTL = 30 * time.Second

type permCacheEntry struct {
	perms     map[string]bool
	expiresAt time.Time
}

var (
	permMu    sync.RWMutex
	permCache = make(map[uint]*permCacheEntry)
)

// GetUserPermissionsCached 带缓存的用户权限查询（中间件用）
// 双层缓存：L1 进程内（30s TTL）+ L2 Redis（多实例共享，redis.enabled=true 时生效）。
// 角色变更时同时失效两层；多实例下 Redis 层保证其他实例即时拿到新权限。
func GetUserPermissionsCached(userID uint) map[string]bool {
	permMu.RLock()
	if e, ok := permCache[userID]; ok && time.Now().Before(e.expiresAt) {
		perms := e.perms
		permMu.RUnlock()
		return perms
	}
	permMu.RUnlock()

	// L2：Redis 命中
	if perms := permCacheFromRedis(userID); perms != nil {
		permMu.Lock()
		permCache[userID] = &permCacheEntry{perms: perms, expiresAt: time.Now().Add(permCacheTTL)}
		permMu.Unlock()
		return perms
	}

	perms := GetUserPermissions(userID)

	permMu.Lock()
	permCache[userID] = &permCacheEntry{perms: perms, expiresAt: time.Now().Add(permCacheTTL)}
	permMu.Unlock()
	permCacheToRedis(userID, perms)
	return perms
}

// permCacheFromRedis 读 L2 缓存（未启用/故障返回 nil 走回源）
func permCacheFromRedis(userID uint) map[string]bool {
	rdb := core.GetRedis()
	if rdb == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	data, err := rdb.Get(ctx, permRedisKey(userID)).Bytes()
	if err != nil {
		return nil
	}
	perms := make(map[string]bool)
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil
	}
	return perms
}

// permCacheToRedis 写 L2 缓存（静默失败：缓存只是加速层）
func permCacheToRedis(userID uint, perms map[string]bool) {
	rdb := core.GetRedis()
	if rdb == nil || perms == nil {
		return
	}
	if data, err := json.Marshal(perms); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_ = rdb.Set(ctx, permRedisKey(userID), data, permCacheTTL).Err()
	}
}

func permRedisKey(userID uint) string {
	return fmt.Sprintf("apeadmin:perm:%d", userID)
}

// InvalidateUserPermCache 失效单个用户的权限缓存
// （用户角色重新分配 / 用户删除 / 密码重置后调用）
func InvalidateUserPermCache(userID uint) {
	permMu.Lock()
	delete(permCache, userID)
	permMu.Unlock()
	if rdb := core.GetRedis(); rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_ = rdb.Del(ctx, permRedisKey(userID)).Err()
	}
}

// InvalidateAllPermCache 全量失效
// （角色变更影响该角色下所有用户，无法精确列举；L1 整体清空 + L2 扫描删除）
func InvalidateAllPermCache() {
	permMu.Lock()
	permCache = make(map[uint]*permCacheEntry)
	permMu.Unlock()
	if rdb := core.GetRedis(); rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		var keys []string
		var cursor uint64
		for {
			page, next, err := rdb.Scan(ctx, cursor, "apeadmin:perm:*", 1000).Result()
			if err != nil {
				break
			}
			keys = append(keys, page...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
		if len(keys) > 0 {
			_ = rdb.Del(ctx, keys...).Err()
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 以下为无缓存原始实现（供缓存层与测试调用）
// ────────────────────────────────────────────────────────────────────────────

// GetUserPermissions 获取用户权限集合
func GetUserPermissions(userID uint) map[string]bool {
	perms := make(map[string]bool)
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		return perms
	}
	for _, role := range user.Roles {
		if role.Status != 1 {
			continue
		}
		menus, err := dal.GetMenusByRoleID(role.ID)
		if err != nil {
			continue
		}
		for _, menu := range menus {
			if menu.Status == 1 && menu.Permission != "" {
				perms[menu.Permission] = true
			}
		}
	}
	return perms
}

// GetUserInfo 获取用户信息（权限码 + 菜单树）
func GetUserInfo(userID uint) (*schema.UserInfo, error) {
	user, err := dal.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	var permissions []string
	var menuNodes []schema.MenuNode

	if user.IsSuperAdmin {
		permissions = []string{"*"}
		menus, err := dal.GetAllMenus()
		if err != nil {
			return nil, err
		}
		menuNodes = buildMenuTree(menus, 0)
	} else {
		perms := GetUserPermissions(userID)
		for p := range perms {
			permissions = append(permissions, p)
		}
		user, err := dal.GetUserWithRoles(userID)
		if err != nil {
			return nil, err
		}
		var allMenus []model.SysMenu
		for _, role := range user.Roles {
			if role.Status != 1 {
				continue
			}
			menus, err := dal.GetMenusByRoleID(role.ID)
			if err != nil {
				continue
			}
			allMenus = append(allMenus, menus...)
		}
		// 去重
		menuMap := make(map[uint]model.SysMenu)
		for _, m := range allMenus {
			if m.Status == 1 {
				menuMap[m.ID] = m
			}
		}
		var deduped []model.SysMenu
		for _, m := range menuMap {
			deduped = append(deduped, m)
		}
		menuNodes = buildMenuTree(deduped, 0)
	}

	roles, _ := dal.GetUserWithRoles(userID)
	roleCodes := make([]string, 0, len(roles.Roles))
	for _, r := range roles.Roles {
		roleCodes = append(roleCodes, r.Code)
	}

	return &schema.UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Permissions: permissions,
		Menus:       menuNodes,
		Roles:       roleCodes,
	}, nil
}

// buildMenuTree 构建菜单树
func buildMenuTree(menus []model.SysMenu, parentID uint) []schema.MenuNode {
	var nodes []schema.MenuNode
	for _, m := range menus {
		if m.ParentID == parentID {
			node := schema.MenuNode{
				ID:         m.ID,
				Name:       m.Name,
				I18nKey:    m.I18nKey,
				ParentID:   m.ParentID,
				Type:       m.Type,
				Path:       m.Path,
				Component:  m.Component,
				Permission: m.Permission,
				Icon:       m.Icon,
				Sort:       m.Sort,
				Visible:    m.Visible,
				Status:     m.Status,
				Children:   buildMenuTree(menus, m.ID),
			}
			nodes = append(nodes, node)
		}
	}
	return nodes
}
