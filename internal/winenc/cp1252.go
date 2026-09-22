package winenc

// windows1252High maps bytes 0x80-0x9F to their Windows-1252 code points
// (the WHATWG "windows-1252" index — the only range where windows-1252
// differs from ISO-8859-1/Latin-1; unassigned positions pass through as
// their own C1 control code, matching the reference implementation's ANSI
// codec, which is windows-1252 by default).
var windows1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

// DecodeWindows1252 decodes ANSI-codec bytes (as EVTX's AnsiStringType
// values are encoded) into a UTF-8 Go string. Embedded NUL bytes should be
// filtered by the caller first, matching the reference implementation's
// AnsiStringType handling.
func DecodeWindows1252(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		if c >= 0x80 && c <= 0x9F {
			runes[i] = windows1252High[c-0x80]
		} else {
			runes[i] = rune(c)
		}
	}
	return string(runes)
}
