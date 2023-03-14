package utils

import (
	"net/http"
	"net/url"
)

func RedirectRequest(req *http.Request, targetURL *url.URL) {
	req.URL.Host = targetURL.Host
	req.URL.Scheme = targetURL.Scheme
	req.URL.Path = SingleJoiningSlash(targetURL.Path, req.URL.Path)

	targetQuery := targetURL.RawQuery
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

	req.Host = req.URL.Host
	req.RequestURI = ""
}
