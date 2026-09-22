// Package winenc converts small Windows-specific encodings (timestamps
// today; GUIDs and SIDs join once BinXML value parsing lands) into their Go
// equivalents.
package winenc

import (
	"fmt"
	"math"
	"time"
)

// filetimeEpochDiff is the number of 100ns intervals between the Windows
// FILETIME epoch (1601-01-01 00:00:00 UTC) and the Unix epoch
// (1970-01-01 00:00:00 UTC).
const filetimeEpochDiff = 116444736000000000

// FileTimeToTime converts a Windows FILETIME value (a count of 100ns
// intervals since 1601-01-01 UTC, as stored in the EVTX record header) into
// a time.Time in UTC.
//
// An error is returned only for filetime values so large they cannot be
// represented as a signed 100ns-interval count; malformed-but-in-range
// values simply produce an implausible (but non-panicking) time, matching
// how the rest of this parser treats untrusted input.
func FileTimeToTime(filetime uint64) (time.Time, error) {
	if filetime > math.MaxInt64 {
		return time.Time{}, fmt.Errorf("filetime %d is out of representable range", filetime)
	}
	hundredNs := int64(filetime) - filetimeEpochDiff
	sec := hundredNs / 10_000_000
	nsec := (hundredNs % 10_000_000) * 100
	return time.Unix(sec, nsec).UTC(), nil
}
