package utils

import (
	"os"
	"strings"
)

func IsExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func TrimStrAndPre(raw, str string) string {
	idx := strings.Index(raw, str)
	if idx < 0 {
		return raw
	}
	if idx+len(str) > len(raw) {
		// impossible
		return raw
	}
	return raw[idx+len(str):]
}
