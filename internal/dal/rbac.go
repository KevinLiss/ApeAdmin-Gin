package dal

import (
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"

	"gorm.io/gorm"
)

// gormDB 是 core.GetDB() 的缓存引用，由 Init 初始化
var gormDB *gorm.DB

// Init 初始化 DAL 层的 DB 引用
func Init() {
	gormDB = core.GetDB()
}

// ─── 角色 ───

func ListRoles(page, pageSize int) ([]model.SysRole, int64, error) {
	var roles []model.SysRole
	var total int64
	db := gormDB.Model(&model.SysRole{})
	db.Count(&total)
	err := db.Preload("Menus").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&roles).Error
	return roles, total, err
}

func ListAllRoles() ([]model.SysRole, error) {
	var roles []model.SysRole
	err := gormDB.Where("status = 1").Find(&roles).Error
	return roles, err
}

func GetRoleByID(id uint) (*model.SysRole, error) {
	var role model.SysRole
	err := gormDB.Preload("Menus").First(&role, id).Error
	return &role, err
}

func CreateRole(role *model.SysRole) error {
	return gormDB.Create(role).Error
}

func UpdateRole(role *model.SysRole) error {
	return gormDB.Save(role).Error
}

func DeleteRole(id uint) error {
	return gormDB.Delete(&model.SysRole{}, id).Error
}

func AssignRoleMenus(roleID uint, menuIDs []uint) error {
	role := model.SysRole{}
	role.ID = roleID
	var menus []model.SysMenu
	gormDB.Where("id IN ?", menuIDs).Find(&menus)
	return gormDB.Model(&role).Association("Menus").Replace(&menus)
}

// ─── 菜单 ───

func GetAllMenus() ([]model.SysMenu, error) {
	var menus []model.SysMenu
	err := gormDB.Where("status = 1").Order("sort ASC").Find(&menus).Error
	return menus, err
}

func GetMenusByRoleID(roleID uint) ([]model.SysMenu, error) {
	var menus []model.SysMenu
	err := gormDB.Raw(`
		SELECT m.* FROM sys_menu m
		INNER JOIN sys_role_menu rm ON rm.menu_id = m.id
		WHERE rm.role_id = ? AND m.status = 1
		ORDER BY m.sort ASC
	`, roleID).Scan(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func GetMenuTree() ([]model.SysMenu, error) {
	return GetAllMenus()
}

func CreateMenu(menu *model.SysMenu) error {
	return gormDB.Create(menu).Error
}

func UpdateMenu(menu *model.SysMenu) error {
	return gormDB.Save(menu).Error
}

func DeleteMenu(id uint) error {
	return gormDB.Delete(&model.SysMenu{}, id).Error
}

// ─── 部门 ───

// GetAllDepts 返回全部部门（含禁用）：管理页需要看到禁用部门才能重新启用；
// 数据权限递归也需要完整树，禁用部门不应导致其子孙部门从数据范围中消失
func GetAllDepts() ([]model.SysDept, error) {
	var depts []model.SysDept
	err := gormDB.Order("sort ASC").Find(&depts).Error
	return depts, err
}

// GetDeptByID 按 ID 查询单个部门
func GetDeptByID(id uint) (*model.SysDept, error) {
	var dept model.SysDept
	err := gormDB.First(&dept, id).Error
	if err != nil {
		return nil, err
	}
	return &dept, nil
}

// CountChildDepts 统计直属子部门数量（用于删除前检查）
func CountChildDepts(parentID uint) (int64, error) {
	var count int64
	err := gormDB.Model(&model.SysDept{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

// CountChildMenus 统计直属子菜单数量（用于删除前检查）
func CountChildMenus(parentID uint) (int64, error) {
	var count int64
	err := gormDB.Model(&model.SysMenu{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

// CountUsersInDept 统计挂在该部门下的用户数量（用于删除前检查）
func CountUsersInDept(deptID uint) (int64, error) {
	var count int64
	err := gormDB.Model(&model.SysUser{}).Where("dept_id = ?", deptID).Count(&count).Error
	return count, err
}

// ExistsDeptName 同一上级下是否存在同名部门（excludeID 用于编辑时排除自身）
func ExistsDeptName(parentID uint, name string, excludeID uint) (bool, error) {
	var count int64
	q := gormDB.Model(&model.SysDept{}).Where("parent_id = ? AND name = ?", parentID, name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func CreateDept(dept *model.SysDept) error {
	return gormDB.Create(dept).Error
}

// IsDeptDescendant 判断 target 是否为 deptID 的子孙（含环检测，避免脏数据导致死循环）
func IsDeptDescendant(deptID, target uint) bool {
	depts, err := GetAllDepts()
	if err != nil {
		return false
	}
	childrenMap := make(map[uint][]uint)
	for _, d := range depts {
		childrenMap[d.ParentID] = append(childrenMap[d.ParentID], d.ID)
	}
	visited := map[uint]bool{deptID: true}
	queue := []uint{deptID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, childID := range childrenMap[curr] {
			if childID == target {
				return true
			}
			if visited[childID] {
				continue // 环防护
			}
			visited[childID] = true
			queue = append(queue, childID)
		}
	}
	return false
}

func UpdateDept(dept *model.SysDept) error {
	return gormDB.Save(dept).Error
}

func DeleteDept(id uint) error {
	return gormDB.Delete(&model.SysDept{}, id).Error
}

// ─── 插件 ───

func ListPlugins(page, pageSize int) ([]model.SysPlugin, int64, error) {
	var plugins []model.SysPlugin
	var total int64
	db := gormDB.Model(&model.SysPlugin{})
	db.Count(&total)
	err := db.Offset((page - 1) * pageSize).Limit(pageSize).Find(&plugins).Error
	return plugins, total, err
}

func GetPluginByID(id uint) (*model.SysPlugin, error) {
	var plugin model.SysPlugin
	err := gormDB.First(&plugin, id).Error
	return &plugin, err
}

func GetPluginByName(name string) (*model.SysPlugin, error) {
	var plugin model.SysPlugin
	err := gormDB.Where("name = ?", name).First(&plugin).Error
	return &plugin, err
}

func CreatePlugin(plugin *model.SysPlugin) error {
	return gormDB.Create(plugin).Error
}

func UpdatePlugin(plugin *model.SysPlugin) error {
	return gormDB.Save(plugin).Error
}

func DeletePlugin(id uint) error {
	return gormDB.Delete(&model.SysPlugin{}, id).Error
}
