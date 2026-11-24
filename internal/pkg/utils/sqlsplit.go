package utils

import (
	"fmt"
	"strings"
)

// SplitSQLStatements 按分号切分 SQL 脚本：跳过注释、尊重引号（字符串内的分号不切分）。
// 供迁移脚本执行器与插件 seed.sql 注入共用，避免各自实现出偏差。
func SplitSQLStatements(sqlText string) ([]string, error) {
	var (
		stmts                          []string
		cur                            strings.Builder
		inSingle, inDouble, inBacktick bool
	)
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]

		// 注释：仅在引号外生效（"--" / "#" 行注释、"/* */" 块注释）
		if !inSingle && !inDouble && !inBacktick {
			if ch == '-' && i+1 < len(sqlText) && sqlText[i+1] == '-' {
				for i < len(sqlText) && sqlText[i] != '\n' {
					i++
				}
				cur.WriteByte('\n')
				continue
			}
			if ch == '#' {
				for i < len(sqlText) && sqlText[i] != '\n' {
					i++
				}
				cur.WriteByte('\n')
				continue
			}
			if ch == '/' && i+1 < len(sqlText) && sqlText[i+1] == '*' {
				i += 2
				for i+1 < len(sqlText) && !(sqlText[i] == '*' && sqlText[i+1] == '/') {
					i++
				}
				i++ // 跳过收尾的 '/'
				cur.WriteByte(' ')
				continue
			}
		}

		switch {
		case ch == '\\' && (inSingle || inDouble) && i+1 < len(sqlText):
			cur.WriteByte(ch)
			i++
			cur.WriteByte(sqlText[i])
			continue
		case ch == '\'' && !inDouble && !inBacktick:
			inSingle = !inSingle
		case ch == '"' && !inSingle && !inBacktick:
			inDouble = !inDouble
		case ch == '`' && !inSingle && !inDouble:
			inBacktick = !inBacktick
		case ch == ';' && !inSingle && !inDouble && !inBacktick:
			flush()
			continue
		}
		cur.WriteByte(ch)
	}

	if inSingle || inDouble || inBacktick {
		return nil, fmt.Errorf("SQL 脚本引号未闭合")
	}
	flush()
	return stmts, nil
}