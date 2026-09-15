package model

import "time"

// SysUser 系统用户
type SysUser struct {
	BaseModel
	Username     string     `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Nickname     string     `gorm:"size:50" json:"nickname"`
	Password     string     `gorm:"size:128;not null" json:"-"`
	Email        string     `gorm:"size:100" json:"email"`
	Phone        string     `gorm:"size:20" json:"phone"`
	Avatar       *string    `gorm:"size:255" json:"avatar"`
	DeptID       *uint      `gorm:"index" json:"dept_id"`
	Status       int        `gorm:"default:1;not null" json:"status"`
	IsSuperAdmin bool       `gorm:"default:false;not null" json:"-"`
	TokenVersion int        `gorm:"default:0;not null" json:"-"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string     `gorm:"size:50" json:"last_login_ip"`
	Roles        []SysRole  `gorm:"many2many:sys_user_role;" json:"roles"`
	Dept         *SysDept   `gorm:"foreignKey:DeptID" json:"dept"`
}

func (SysUser) TableName() string { return "sys_user" }

// SysRole 系统角色
type SysRole struct {
	BaseModel
	Name      string    `gorm:"size:50;not null" json:"name"`
	Code      string    `gorm:"uniqueIndex;size:50;not null" json:"code"`
	DataScope int       `gorm:"default:1" json:"data_scope"`
	Sort      int       `gorm:"default:0" json:"sort"`
	Status    int       `gorm:"default:1;not null" json:"status"`
	Remark    string    `gorm:"size:200" json:"remark"`
	Menus     []SysMenu `gorm:"many2many:sys_role_menu;" json:"menus"`
}

func (SysRole) TableName() string { return "sys_role" }

// SysMenu 系统菜单
type SysMenu struct {
	BaseModel
	Name       string `gorm:"size:50;not null" json:"name"`
	ParentID   uint   `gorm:"default:0;index" json:"parent_id"`
	Type       string `gorm:"size:1;not null" json:"type"` // M=目录 C=菜单 F=按钮
	Path       string `gorm:"size:200" json:"path"`
	Component  string `gorm:"size:200" json:"component"`
	Permission string `gorm:"size:100" json:"permission"`
	Icon       string `gorm:"size:50" json:"icon"`
	Sort       int    `gorm:"default:0" json:"sort"`
	Visible    int    `gorm:"default:1" json:"visible"`
	Status     int    `gorm:"default:1" json:"status"`
}

func (SysMenu) TableName() string { return "sys_menu" }

// SysDept 系统部门
type SysDept struct {
	BaseModel
	Name     string `gorm:"size:50;not null" json:"name"`
	ParentID uint   `gorm:"default:0;index" json:"parent_id"`
	Sort     int    `gorm:"default:0" json:"sort"`
	Leader   string `gorm:"size:50" json:"leader"`
	Phone    string `gorm:"size:20" json:"phone"`
	Status   int    `gorm:"default:1;not null" json:"status"`
}

func (SysDept) TableName() string { return "sys_dept" }
