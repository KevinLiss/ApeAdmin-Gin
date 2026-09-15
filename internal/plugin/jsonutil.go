package plugin

import "encoding/json"

// jsonUnmarshal 包装 json.Unmarshal（方便测试 mock）
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
