package utils

import "strings"

// 比较两个字符串是否相等，忽略末尾的换行符
func StringsEqualIgnoreFinalNewline(a, b string) bool {

	return trimNewline(a) == trimNewline(b)
}

// 去除两个字符串末尾的换行符（兼容不同系统的换行符）
func trimNewline(s string) string {
	return strings.TrimRight(s, "\r\n")
}
