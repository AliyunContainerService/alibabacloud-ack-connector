package utils

import "path"

func SingleJoiningSlash(a, b string) string {
	if a == "" || a == "/" {
		return b
	}
	if b == "" || b == "/" {
		return a
	}

	return path.Join(a, b)
}
