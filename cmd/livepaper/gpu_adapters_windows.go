//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// displayClassGUID is the device setup class for "Display adapters".
const displayClassGUID = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`

// enumerateGPUAdapters returns the human-readable names of the installed
// display adapters (e.g. "NVIDIA GeForce RTX 4070", "Intel UHD Graphics 770").
func enumerateGPUAdapters() []string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, displayClassGUID, registry.READ)
	if err != nil {
		return nil
	}
	defer k.Close()

	subs, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	for _, sub := range subs {
		// Adapter instances are 4-digit numeric subkeys (0000, 0001, ...).
		if len(sub) != 4 {
			continue
		}
		sk, err := registry.OpenKey(registry.LOCAL_MACHINE, displayClassGUID+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		desc, _, err := sk.GetStringValue("DriverDesc")
		sk.Close()
		if err != nil || desc == "" || seen[desc] {
			continue
		}
		seen[desc] = true
		out = append(out, desc)
	}
	return out
}
