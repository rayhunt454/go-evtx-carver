package winenc

import "strconv"

// FormatSID formats raw Windows SID bytes ("S-R-A-S1-..-Sn" form): a
// 1-byte revision, a 1-byte sub-authority count, a 48-bit big-endian
// IdentifierAuthority, then that many 4-byte little-endian sub-authorities.
// Malformed/truncated input degrades gracefully rather than panicking,
// matching how the rest of this parser treats untrusted data.
func FormatSID(b []byte) string {
	if len(b) < 8 {
		return "S-?"
	}
	revision := b[0]
	subCount := int(b[1])

	var authority uint64
	for _, v := range b[2:8] {
		authority = authority<<8 | uint64(v)
	}

	out := "S-" + strconv.FormatUint(uint64(revision), 10) + "-" + strconv.FormatUint(authority, 10)

	off := 8
	for i := 0; i < subCount; i++ {
		if off+4 > len(b) {
			break
		}
		sub := uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
		out += "-" + strconv.FormatUint(uint64(sub), 10)
		off += 4
	}
	return out
}
