package plugin

import "archive/zip"

// zipOpenReader 包装 zip.OpenReader（方便测试 mock）
func zipOpenReader(path string) (*zip.ReadCloser, error) {
	return zip.OpenReader(path)
}
