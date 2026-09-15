package tree

// Node 通用树节点接口
type Node interface {
	GetID() uint
	GetParentID() uint
}

// Build 通用树构建器
// 用法：tree.Build(items, 0) — items 需实现 Node 接口
func Build[T Node](items []T, rootParentID uint) []T {
	itemMap := make(map[uint][]T)
	for _, item := range items {
		itemMap[item.GetParentID()] = append(itemMap[item.GetParentID()], item)
	}

	var build func(parentID uint) []T
	build = func(parentID uint) []T {
		children := itemMap[parentID]
		return children
	}

	return build(rootParentID)
}
