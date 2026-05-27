//go:build !livepaper_dev

package main

import "io/fs"

// readIconFile reads an icon from the embedded dist filesystem.
// Astro copies public/ into dist/ verbatim, so icon.ico, icon-32.png etc. are available.
func readIconFile(name string) []byte {
	data, err := fs.ReadFile(distFS, "dist/"+name)
	if err != nil {
		return nil
	}
	return data
}
