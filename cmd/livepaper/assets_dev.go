//go:build livepaper_dev

package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

const astroDevURL = "http://localhost:4321"

func getAssetHandler() http.Handler {
	target, _ := url.Parse(astroDevURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}
