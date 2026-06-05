//go:build windows

package main

import (
	"testing"

	wp "github.com/dvgamerr/go-livepaper/internal/wallpaper"
)

// makeMonitors returns a slice of n generic MonitorInfo entries for testing.
func makeMonitors(n int) []wp.MonitorInfo {
	monitors := make([]wp.MonitorInfo, n)
	for i := range monitors {
		monitors[i] = wp.MonitorInfo{
			Index:   i,
			Primary: i == 0,
			Resolution: wp.Resolution{
				Width:  1920,
				Height: 1080,
				X:      int32(i * 1920),
				Y:      0,
			},
		}
	}
	return monitors
}

func TestSelectMonitors_AllWhenEmpty(t *testing.T) {
	monitors := makeMonitors(3)
	got, err := selectMonitors(monitors, nil)
	if err != nil {
		t.Fatalf("selectMonitors(nil selected) error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}

func TestSelectMonitors_SpecificMonitors(t *testing.T) {
	monitors := makeMonitors(3)
	got, err := selectMonitors(monitors, []string{"2", "1"})
	if err != nil {
		t.Fatalf("selectMonitors([2,1]) error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// "2" picks monitors[1] (1-based), "1" picks monitors[0].
	if got[0].Index != 1 {
		t.Errorf("got[0].Index = %d, want 1", got[0].Index)
	}
	if got[1].Index != 0 {
		t.Errorf("got[1].Index = %d, want 0", got[1].Index)
	}
}

func TestSelectMonitors_SingleMonitor(t *testing.T) {
	monitors := makeMonitors(2)
	got, err := selectMonitors(monitors, []string{"1"})
	if err != nil {
		t.Fatalf("selectMonitors([1]) error = %v", err)
	}
	if len(got) != 1 || got[0].Index != 0 {
		t.Errorf("got = %v, want single monitor with Index=0", got)
	}
}

func TestSelectMonitors_InvalidNumber(t *testing.T) {
	monitors := makeMonitors(2)
	_, err := selectMonitors(monitors, []string{"abc"})
	if err == nil {
		t.Error("selectMonitors(non-numeric): expected error")
	}
}

func TestSelectMonitors_OutOfRange_Zero(t *testing.T) {
	monitors := makeMonitors(2)
	_, err := selectMonitors(monitors, []string{"0"})
	if err == nil {
		t.Error("selectMonitors(0): expected error (1-based, 0 is invalid)")
	}
}

func TestSelectMonitors_OutOfRange_TooHigh(t *testing.T) {
	monitors := makeMonitors(2)
	_, err := selectMonitors(monitors, []string{"3"})
	if err == nil {
		t.Error("selectMonitors(3) with 2 monitors: expected error")
	}
}

func TestSelectMonitors_Duplicate(t *testing.T) {
	monitors := makeMonitors(3)
	_, err := selectMonitors(monitors, []string{"1", "1"})
	if err == nil {
		t.Error("selectMonitors(duplicate): expected error")
	}
}

func TestSelectMonitors_EmptyMonitorList(t *testing.T) {
	got, err := selectMonitors(nil, nil)
	if err != nil {
		t.Fatalf("selectMonitors(nil,nil) error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
