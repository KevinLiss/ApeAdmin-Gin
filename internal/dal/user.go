package dal

import (
	"gin-apeadmin/internal/model"

	"gorm.io/gorm"
)

// GetUserByID 根据 ID 查询用户（不含关联）
func GetUserByID(id uint) (*model.SysUser, error) {
	db := getDB()
	var user model.SysUser
	err := db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername 根据用户名查询用户（含角色）
func GetUserByUsername(username string) (*model.SysUser, error) {
	db := getDB()
	var user model.SysUser
	err := db.Preload("Roles").Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserWithRoles 查询用户及其角色
func GetUserWithRoles(id uint) (*model.SysUser, error) {
	db := getDB()
	var user model.SysUser
	err := db.Preload("Roles").Preload("Dept").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers 分页查询用户
func ListUsers(page, pageSize int, keyword string) ([]model.SysUser, int64, error) {
	db := getDB()
	var users []model.SysUser
	var total int64

	query := db.Model(&model.SysUser{})
	if keyword != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	err := query.Preload("Roles").Preload("Dept").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&users).Error
	return users, total, err
}

// CreateUser 创建用户
func CreateUser(user *model.SysUser) error {
	return getDB().Create(user).Error
}

// UpdateUser 更新用户
func UpdateUser(user *model.SysUser) error {
	return getDB().Save(user).Error
}

// DeleteUser 软删除用户
func DeleteUser(id uint) error {
	return getDB().Delete(&model.SysUser{}, id).Error
}

// UpdateUserPassword 更新用户密码
func UpdateUserPassword(id uint, hash string) error {
	return getDB().Model(&model.SysUser{}).Where("id = ?", id).Update("password", hash).Error
}

// IncrementTokenVersion 递增 TokenVersion（改密/禁用时调用）
func IncrementTokenVersion(id uint) error {
	return getDB().Model(&model.SysUser{}).Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

// UpdateUserStatus 更新用户状态
func UpdateUserStatus(id uint, status int) error {
	return getDB().Model(&model.SysUser{}).Where("id = ?", id).Update("status", status).Error
}

// AssignUserRoles 分配用户角色
func AssignUserRoles(userID uint, roleIDs []uint) error {
	user := model.SysUser{}
	user.ID = userID
	var roles []model.SysRole
	getDB().Where("id IN ?", roleIDs).Find(&roles)
	return getDB().Model(&user).Association("Roles").Replace(&roles)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return gormDB
}

func getDB() *gorm.DB {
	return gormDB
}
