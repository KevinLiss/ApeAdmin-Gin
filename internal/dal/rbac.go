package dal

import (
	"gin-apeadmin/internal/core"
	"gin-apeadmin/internal/model"

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

func GetAllDepts() ([]model.SysDept, error) {
	var depts []model.SysDept
	err := gormDB.Where("status = 1").Order("sort ASC").Find(&depts).Error
	return depts, err
}

func CreateDept(dept *model.SysDept) error {
	return gormDB.Create(dept).Error
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
