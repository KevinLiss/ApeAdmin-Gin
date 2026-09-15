package dal

import (
	"gin-apeadmin/internal/model"

	"gorm.io/gorm"
)

// DataScope 数据权限范围常量
const (
	DataScopeSelf          = 1 // 本人
	DataScopeDeptAndChildren = 2 // 本部门及以下
	DataScopeDeptOnly      = 3 // 本部门
	DataScopeAll           = 4 // 全部
)

// DataScopeScope 返回 GORM Scope：按用户角色的 data_scope 过滤当前查询
// 用法：db.Scopes(dal.DataScopeScope(user, "creator_id", "dept_id")).Find(&items)
func DataScopeScope(u *model.SysUser, ownerColumn, deptColumn string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if u.IsSuperAdmin {
			return db
		}
		scope := maxDataScope(u.Roles)
		switch scope {
		case DataScopeAll:
			return db
		case DataScopeDeptAndChildren:
			deptIDs := descendantDeptIDs(u.DeptID)
			if len(deptIDs) == 0 {
				return db.Where("1 = 0") // 无部门数据
			}
			return db.Where(deptColumn+" IN ?", deptIDs)
		case DataScopeDeptOnly:
			if u.DeptID == nil {
				return db.Where("1 = 0")
			}
			return db.Where(deptColumn+" = ?", *u.DeptID)
		default:
			return db.Where(ownerColumn+" = ?", u.ID)
		}
	}
}

// maxDataScope 多角色取最大范围（值越大范围越广）
func maxDataScope(roles []model.SysRole) int {
	max := DataScopeSelf
	for _, r := range roles {
		if r.Status != 1 {
			continue
		}
		if r.DataScope > max {
			max = r.DataScope
		}
	}
	return max
}

// descendantDeptIDs 获取部门及其子部门 ID 集合（内存缓存可后续优化）
func descendantDeptIDs(deptID *uint) []uint {
	if deptID == nil {
		return nil
	}
	depts, err := GetAllDepts()
	if err != nil || len(depts) == 0 {
		return []uint{*deptID}
	}
	// BFS 收集子树
	result := []uint{*deptID}
	childrenMap := make(map[uint][]uint)
	for _, d := range depts {
		childrenMap[d.ParentID] = append(childrenMap[d.ParentID], d.ID)
	}
	queue := []uint{*deptID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, childID := range childrenMap[curr] {
			result = append(result, childID)
			queue = append(queue, childID)
		}
	}
	return result
}
