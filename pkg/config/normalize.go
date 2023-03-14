package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func getAddress(addr string) (string, error) {
	// normalize port to addr
	if _, err := strconv.Atoi(addr); err == nil {
		addr = ":" + addr
	}

	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	if h == "" {
		h = "127.0.0.1"
	}

	return fmt.Sprintf("%s:%s", h, p), nil
}

func getURL(rawurl string) (string, error) {
	list := strings.SplitN(rawurl, "://", 2)
	if len(list) > 1 {
		if list[0] != HTTP || list[0] != HTTPS {
			return "", fmt.Errorf("unsupported url schema, only 'http' or 'https' is allowed")
		}
	} else {
		rawurl = fmt.Sprint("http://", rawurl)
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}

	if u.Path != "" && !strings.HasSuffix(u.Path, "/") {
		return "", fmt.Errorf("url must end with '/'")
	}

	return rawurl, nil
}
