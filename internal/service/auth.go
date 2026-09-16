package service

import (
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/schema"
)

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
