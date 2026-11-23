package acquisition

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvt-project/androidqf/adb"
)

func TestMeasureDeviceClock(t *testing.T) {
	base := time.Unix(1700000000, 0)
	for _, tt := range []struct {
		name       string
		output     string
		wantOffset float64
	}{
		{"ahead", "1700000010.000000000\r\n", 9.9000000005},
		{"behind", "1699999990.000000000", -10.0999999995},
		{"far future", "253402300799.000000000", 251702300798.9},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			clock, err := measureDeviceClock(func(args ...string) (string, error) {
				if strings.Join(args, " ") != "date -u +%s.%N" {
					t.Fatalf("command = %v", args)
				}
				return tt.output, nil
			}, func() time.Time {
				result := base.Add(time.Duration(calls) * 200 * time.Millisecond)
				calls++
				return result
			})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(clock.OffsetSeconds-tt.wantOffset) > 1e-6 {
				t.Fatalf("offset = %v, want %v", clock.OffsetSeconds, tt.wantOffset)
			}
			if clock.ResolutionSeconds != 1e-9 || math.Abs(clock.UncertaintySeconds-0.1000000005) > 1e-12 {
				t.Fatalf("clock = %+v", clock)
			}
			if !clock.HostBefore.Equal(base) || !clock.HostAfter.Equal(base.Add(200*time.Millisecond)) {
				t.Fatalf("host interval = %+v", clock)
			}
		})
	}
}

func TestMeasureDeviceClockFallback(t *testing.T) {
	for _, firstErr := range []error{nil, errors.New("unsupported format")} {
		t.Run(fmtErrorName(firstErr), func(t *testing.T) {
			base := time.Unix(1700000000, 0)
			moments := []time.Time{base, base.Add(2 * time.Second), base.Add(3 * time.Second), base.Add(3200 * time.Millisecond)}
			calls := 0
			clock, err := measureDeviceClock(func(args ...string) (string, error) {
				if args[2] == "+%s.%N" {
					return "1700000010.%N", firstErr
				}
				if args[2] != "+%s" {
					t.Fatalf("fallback = %v", args)
				}
				return "1700000010", nil
			}, func() time.Time { result := moments[calls]; calls++; return result })
			if err != nil {
				t.Fatal(err)
			}
			if clock.ResolutionSeconds != 1 || math.Abs(clock.OffsetSeconds-7.4) > 1e-12 || clock.UncertaintySeconds != 0.6 {
				t.Fatalf("clock = %+v", clock)
			}
			if !clock.HostBefore.Equal(moments[2]) || !clock.HostAfter.Equal(moments[3]) {
				t.Fatalf("fallback reused first interval: %+v", clock)
			}
		})
	}
}

func fmtErrorName(err error) string {
	if err == nil {
		return "literal format"
	}
	return "command failure"
}

func TestMeasureDeviceClockFailure(t *testing.T) {
	for _, tt := range []struct {
		name, output string
		err          error
	}{
		{"failed commands", "1700000000", errors.New("device offline")},
		{"malformed output", "not a date", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			clock, err := measureDeviceClock(func(...string) (string, error) { calls++; return tt.output, tt.err }, time.Now)
			if clock != nil || err == nil || calls != 2 {
				t.Fatalf("clock = %v, err = %v, calls = %d", clock, err, calls)
			}
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("lost command error: %v", err)
			}
		})
	}
}

func TestMeasureDeviceClockRejectsHostClockChange(t *testing.T) {
	// The same behavior is testable without monotonic timestamps by making
	// the host clock run backwards during the request.
	base := time.Unix(1700000000, 0)
	calls := 0
	clock, err := measureDeviceClock(func(...string) (string, error) { return "1700000000.000000000", nil }, func() time.Time {
		result := base.Add(-time.Duration(calls) * time.Second)
		calls++
		return result
	})
	if clock != nil || err == nil || !strings.Contains(err.Error(), "host clock changed") {
		t.Fatalf("clock = %v, error = %v", clock, err)
	}
}

func TestParseDeviceTimeRejectsInvalidOutput(t *testing.T) {
	for _, value := range []string{"", "date: bad option", "1700000000.%N", "1700000000", "1700000000.1", "1700000000.00000000x", "1700000000.+00000000", "1700000000.000000000\nother", "253402300800.000000000", "9223372036854775808.000000000", "-1.000000000"} {
		if _, _, err := parseDeviceTime(value, true); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"", "1700000000.000000000", "+1700000000", "-1", "253402300800"} {
		if _, _, err := parseDeviceTime(value, false); err == nil {
			t.Errorf("accepted whole seconds %q", value)
		}
	}
}

func runClockADBHelper() error {
	args := os.Args[1:]
	if len(args) == 0 {
		return errors.New("missing adb command")
	}
	if args[0] == "pubkey" {
		fmt.Println("test-public-key")
		return nil
	}
	if len(args) < 3 || args[0] != "-s" || args[1] != "clock-device" {
		return fmt.Errorf("missing selected device: %v", args)
	}
	args = args[2:]
	switch strings.Join(args, " ") {
	case "shell getprop ro.product.cpu.abi":
		// Avoid uploading a collector: clock acquisition must not depend on it.
		fmt.Println("unsupported-test-architecture")
	case "shell env":
		fmt.Println("TMPDIR=/data/local/tmp")
	case "shell date -u +%s.%N", "shell date -u +%s":
		if os.Getenv("ANDROIDQF_CLOCK_ADB_FAIL") == "1" {
			return errors.New("date unavailable")
		}
		fmt.Println(os.Getenv("ANDROIDQF_CLOCK_ADB_OUTPUT"))
	default:
		// Collector existence check and server cleanup succeed with no output.
	}
	return nil
}

func TestNewRecordsDeviceClockWithoutMakingFailuresFatal(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, output, fail string
		wantClock          bool
	}{
		{"success", "1700000000.123456789", "0", true},
		{"malformed", "invalid", "0", false},
		{"command failure", "", "1", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("ANDROIDQF_CLOCK_ADB_HELPER", "1")
			t.Setenv("ANDROIDQF_CLOCK_ADB_OUTPUT", tt.output)
			t.Setenv("ANDROIDQF_CLOCK_ADB_FAIL", tt.fail)
			original := adb.Client
			adb.Client = &adb.ADB{ExePath: executable, Serial: "clock-device"}
			t.Cleanup(func() { adb.Client = original })
			acq, err := New(dir)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			if err := acq.Complete(); err != nil {
				t.Fatalf("Complete() failed: %v", err)
			}
			files := readZipFiles(t, filepath.Join(dir, acq.UUID+".zip"))
			if (acq.DeviceClock != nil) != tt.wantClock {
				t.Fatalf("device clock = %+v", acq.DeviceClock)
			}
			if strings.Contains(files["acquisition.json"], "\"device_clock\"") != tt.wantClock {
				t.Fatalf("clock metadata presence incorrect: %s", files["acquisition.json"])
			}
			if !tt.wantClock && !strings.Contains(files["command.log"], "Unable to measure device clock") {
				t.Fatalf("missing archived warning: %s", files["command.log"])
			}
		})
	}
}
