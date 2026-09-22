package winenc

import (
	"fmt"
	"time"
)

// SysTimeFromBytes decodes a Windows SYSTEMTIME structure (16 bytes: 8
// little-endian uint16 fields — year, month, day-of-week, day, hour,
// minute, second, milliseconds; day-of-week is ignored, matching the
// reference implementation) into a UTC time.Time.
func SysTimeFromBytes(b [16]byte) (time.Time, error) {
	u16 := func(off int) uint16 { return uint16(b[off]) | uint16(b[off+1])<<8 }

	year := u16(0)
	month := u16(2)
	// dayOfWeek := u16(4) // unused
	day := u16(6)
	hour := u16(8)
	minute := u16(10)
	second := u16(12)
	millis := u16(14)

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid SYSTEMTIME: year=%d month=%d day=%d", year, month, day)
	}

	return time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second),
		int(millis)*1_000_000, time.UTC), nil
}
