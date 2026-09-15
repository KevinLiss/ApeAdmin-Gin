package model

// SysPlugin 插件注册表
type SysPlugin struct {
	BaseModel
	Name        string  `gorm:"uniqueIndex;size:100;not null" json:"name"`
	DisplayName string  `gorm:"size:200" json:"display_name"`
	Description string  `gorm:"size:500" json:"description"`
	Version     string  `gorm:"size:20" json:"version"`
	Author      string  `gorm:"size:100" json:"author"`
	ModulePath  string  `gorm:"size:255" json:"module_path"`
	Enabled     bool    `gorm:"default:false" json:"enabled"`
	Config      *string `gorm:"type:text" json:"config"`
}

func (SysPlugin) TableName() string { return "sys_plugin" }
