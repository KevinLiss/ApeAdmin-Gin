package pagination

import (
	"gorm.io/gorm"
)

// Params 分页参数
type Params struct {
	Page     int `form:"page" binding:"min=1"`
	PageSize int `form:"page_size" binding:"min=1,max=100"`
}

// GetPage 获取页码（默认1）
func (p *Params) GetPage() int {
	if p.Page == 0 {
		return 1
	}
	return p.Page
}

// GetPageSize 获取每页条数（默认20，最大100）
func (p *Params) GetPageSize() int {
	if p.PageSize == 0 {
		return 20
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

// Offset 计算偏移量
func (p *Params) Offset() int {
	return (p.GetPage() - 1) * p.GetPageSize()
}

// Paginate GORM 分页 Scope
func (p *Params) Paginate(db *gorm.DB) *gorm.DB {
	return db.Offset(p.Offset()).Limit(p.GetPageSize())
}
