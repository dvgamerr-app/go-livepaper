//go:build !windows

package main

func checkSingleInstance() bool {
	return true
}
