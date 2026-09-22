package winenc

const hexDigitsUpper = "0123456789ABCDEF"

// FormatGUID formats a 16-byte Windows GUID in its canonical mixed-endian,
// uppercase-hex, dashed form ("D1-D2-D3-D4[0:2]-D4[2:8]", with Data1..Data3
// stored little-endian on the wire and byte-reversed here), matching the
// reference implementation's GUID Display output.
func FormatGUID(b [16]byte) string {
	order := [16]int{3, 2, 1, 0, 5, 4, 7, 6, 8, 9, 10, 11, 12, 13, 14, 15}
	out := make([]byte, 36)
	for i := range out {
		out[i] = '-'
	}
	n := 0
	for i, idx := range order {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			n++
		}
		v := b[idx]
		out[n] = hexDigitsUpper[v>>4]
		out[n+1] = hexDigitsUpper[v&0x0f]
		n += 2
	}
	return string(out)
}
