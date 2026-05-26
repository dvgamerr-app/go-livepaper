//go:build !livepaper_dev

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

func getAssetHandler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(dist))
}
