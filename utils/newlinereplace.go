package utils

import "strings"

// ReplaceNewLine replaces the \r and \n with a space
func ReplaceNewLine(s string) string {
	x := strings.Replace(s, "\r", " ", -1)
	x = strings.Replace(x, "\n", " ", -1)
	return x
}
