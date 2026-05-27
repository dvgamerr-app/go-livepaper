//go:build windows

package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// GPUStats holds current GPU utilization and dedicated VRAM used.
type GPUStats struct {
	UsagePct float64 `json:"usagePct"`
	UsedMB   int64   `json:"usedMB"`
}

var (
	pdh                              = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery                 = pdh.NewProc("PdhOpenQuery")
	procPdhAddEnglishCounterW        = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData          = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterArrayW = pdh.NewProc("PdhGetFormattedCounterArrayW")
	procPdhCloseQuery                = pdh.NewProc("PdhCloseQuery")
)

const (
	pdhFmtDouble = 0x00000200
	pdhMoreData  = 0x800007D2
)

// mirrors PDH_FMT_COUNTERVALUE for DOUBLE type
type pdhCounterValueDouble struct {
	CStatus uint32
	_       [4]byte
	Value   float64
}

// mirrors PDH_FMT_COUNTERVALUE_ITEM_W for DOUBLE type
type pdhCounterValueItemDouble struct {
	SzName   *uint16
	FmtValue pdhCounterValueDouble
}

var gpuState struct {
	mu    sync.RWMutex
	query uintptr
	hGPU  uintptr
	hVRAM uintptr
	ready bool
}

// initGPU opens a persistent PDH query for GPU stats. Call once in a goroutine.
func initGPU() {
	var q uintptr
	if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&q))); r != 0 {
		return
	}

	gpuPath, _ := syscall.UTF16PtrFromString(`\GPU Engine(*engtype_3D)\Utilization Percentage`)
	var hGPU uintptr
	procPdhAddEnglishCounterW.Call(q, uintptr(unsafe.Pointer(gpuPath)), 0, uintptr(unsafe.Pointer(&hGPU)))

	vramPath, _ := syscall.UTF16PtrFromString(`\GPU Adapter Memory(*)\Dedicated Usage`)
	var hVRAM uintptr
	procPdhAddEnglishCounterW.Call(q, uintptr(unsafe.Pointer(vramPath)), 0, uintptr(unsafe.Pointer(&hVRAM)))

	// Rate counters need a baseline collection before they return meaningful values.
	procPdhCollectQueryData.Call(q)
	time.Sleep(500 * time.Millisecond)
	procPdhCollectQueryData.Call(q)

	gpuState.mu.Lock()
	gpuState.query = q
	gpuState.hGPU = hGPU
	gpuState.hVRAM = hVRAM
	gpuState.ready = true
	gpuState.mu.Unlock()
}

// readGPUStats collects and returns the latest GPU stats.
func readGPUStats() GPUStats {
	gpuState.mu.RLock()
	ready := gpuState.ready
	q, hGPU, hVRAM := gpuState.query, gpuState.hGPU, gpuState.hVRAM
	gpuState.mu.RUnlock()

	if !ready {
		return GPUStats{}
	}

	procPdhCollectQueryData.Call(q)

	pct := pdhArraySum(hGPU)
	if pct > 100 {
		pct = 100
	}

	return GPUStats{
		UsagePct: pct,
		UsedMB:   int64(pdhArraySum(hVRAM)) / (1024 * 1024),
	}
}

// pdhArraySum returns the sum of all valid instance values in a formatted counter array.
func pdhArraySum(handle uintptr) float64 {
	if handle == 0 {
		return 0
	}
	var size, count uint32
	r, _, _ := procPdhGetFormattedCounterArrayW.Call(
		handle, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Pointer(&count)),
		0,
	)
	if r != pdhMoreData || size == 0 {
		return 0
	}

	buf := make([]byte, size)
	r, _, _ = procPdhGetFormattedCounterArrayW.Call(
		handle, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r != 0 {
		return 0
	}

	stride := unsafe.Sizeof(pdhCounterValueItemDouble{})
	var sum float64
	for i := uint32(0); i < count; i++ {
		item := (*pdhCounterValueItemDouble)(unsafe.Pointer(&buf[uintptr(i)*stride]))
		if item.FmtValue.CStatus == 0 {
			sum += item.FmtValue.Value
		}
	}
	return sum
}
