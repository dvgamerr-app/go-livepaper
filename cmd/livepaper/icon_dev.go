//go:build livepaper_dev

package main

import "os"

// readIconFile reads an icon from the public/ directory on disk.
// Assumes CWD is the project root, which is the normal dev working directory.
func readIconFile(name string) []byte {
	data, err := os.ReadFile("public/" + name)
	if err != nil {
		return nil
	}
	return data
}
