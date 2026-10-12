// androidqf - Android Quick Forensics
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DeviceClock records one device wall-clock observation and its host request
// interval. OffsetSeconds is device minus host: positive means the device is
// ahead. The estimate includes half the timestamp's truncation interval.
type DeviceClock struct {
	DeviceTime         time.Time `json:"device_time"`
	HostBefore         time.Time `json:"host_before"`
	HostAfter          time.Time `json:"host_after"`
	ResolutionSeconds  float64   `json:"resolution_seconds"`
	RoundTripSeconds   float64   `json:"round_trip_seconds"`
	OffsetSeconds      float64   `json:"offset_seconds"`
	UncertaintySeconds float64   `json:"uncertainty_seconds"`
}

func measureDeviceClock(shell func(...string) (string, error), now func() time.Time) (*DeviceClock, error) {
	var attempts []error
	// Some Android date implementations do not support %N. Each attempt needs
	// its own host interval, including the whole-second fallback.
	for _, format := range []string{"+%s.%N", "+%s"} {
		before := now()
		output, err := shell("date", "-u", format)
		after := now()
		if err != nil {
			attempts = append(attempts, fmt.Errorf("date %s: %w", format, err))
			continue
		}
		deviceTime, resolution, err := parseDeviceTime(output, format == "+%s.%N")
		if err != nil {
			attempts = append(attempts, err)
			continue
		}

		roundTrip := after.Sub(before)
		wallInterval := after.Round(0).Sub(before.Round(0))
		// A wall-clock adjustment invalidates the midpoint estimate. Compare
		// against the monotonic interval when time.Now supplies one.
		if roundTrip < 0 || wallInterval < 0 || (wallInterval-roundTrip).Abs() > time.Millisecond {
			return nil, errors.New("host clock changed during device clock measurement")
		}
		midpoint := before.Add(roundTrip / 2).UTC()
		// Use Unix seconds rather than Time.Sub so a badly wrong device clock
		// cannot saturate a time.Duration and silently cap the offset.
		offset := float64(deviceTime.Unix()-midpoint.Unix()) +
			float64(deviceTime.Nanosecond()-midpoint.Nanosecond())/1e9 + resolution.Seconds()/2
		return &DeviceClock{
			DeviceTime:         deviceTime,
			HostBefore:         before.UTC(),
			HostAfter:          after.UTC(),
			ResolutionSeconds:  resolution.Seconds(),
			RoundTripSeconds:   roundTrip.Seconds(),
			OffsetSeconds:      offset,
			UncertaintySeconds: (roundTrip.Seconds() + resolution.Seconds()) / 2,
		}, nil
	}
	return nil, errors.Join(attempts...)
}

func parseDeviceTime(output string, fractional bool) (time.Time, time.Duration, error) {
	value := strings.TrimSpace(output)
	seconds, fraction, hasFraction := strings.Cut(value, ".")
	if seconds == "" || hasFraction != fractional || (fractional && len(fraction) != 9) {
		return time.Time{}, 0, errors.New("invalid device date output")
	}
	// Limit timestamps to years representable in RFC3339 JSON, also keeping
	// offset arithmetic within int64. Android date prints decimal epoch seconds.
	for _, c := range seconds {
		if c < '0' || c > '9' {
			return time.Time{}, 0, errors.New("invalid device epoch seconds")
		}
	}
	sec, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil || sec > 253402300799 {
		return time.Time{}, 0, errors.New("device epoch seconds out of range")
	}
	var ns int64
	resolution := time.Second
	if fractional {
		for _, c := range fraction {
			if c < '0' || c > '9' {
				return time.Time{}, 0, errors.New("invalid device fractional seconds")
			}
		}
		ns, _ = strconv.ParseInt(fraction, 10, 64)
		resolution = time.Nanosecond
	}
	return time.Unix(sec, ns).UTC(), resolution, nil
}
