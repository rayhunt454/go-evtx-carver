package binxml

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

type Utf16Slice struct {
	Bytes []byte // always even length
	Chars int    // number of valid UTF-16 code units at the start of Bytes
}

func (s Utf16Slice) IsEmpty() bool { return s.Chars == 0 }

const (
	surrogateHiStart = 0xd800
	surrogateLoStart = 0xdc00
	surrogateEnd     = 0xe000
)

func (s Utf16Slice) String() string {
	if s.Chars == 0 {
		return ""
	}
	if out, ok := s.asciiFastPath(); ok {
		return out
	}

	var sb strings.Builder
	sb.Grow(s.Chars * 3) // typical non-ASCII BMP char is 2-3 UTF-8 bytes
	for i := 0; i < s.Chars; i++ {
		r := rune(uint16(s.Bytes[2*i]) | uint16(s.Bytes[2*i+1])<<8)
		switch {
		case r < surrogateHiStart || surrogateEnd <= r:
			sb.WriteRune(r)
		case r < surrogateLoStart && i+1 < s.Chars:
			r2 := rune(uint16(s.Bytes[2*(i+1)]) | uint16(s.Bytes[2*(i+1)+1])<<8)
			if surrogateLoStart <= r2 && r2 < surrogateEnd {
				sb.WriteRune(utf16.DecodeRune(r, r2))
				i++
			} else {
				sb.WriteRune(utf8.RuneError)
			}
		default:
			sb.WriteRune(utf8.RuneError)
		}
	}
	return sb.String()
}

func (s Utf16Slice) asciiFastPath() (string, bool) {
	for i := 0; i < s.Chars; i++ {
		if s.Bytes[2*i+1] != 0 || s.Bytes[2*i] >= 0x80 {
			return "", false
		}
	}
	out := make([]byte, s.Chars)
	for i := 0; i < s.Chars; i++ {
		out[i] = s.Bytes[2*i]
	}
	return string(out), true
}

func effectiveChars(b []byte) int {
	n := len(b) / 2
	for i := 0; i < n; i++ {
		if b[2*i] == 0 && b[2*i+1] == 0 {
			return i
		}
	}
	return n
}

func readUtf16ByCharCount(c *bytesutil.Cursor, charCount int) (Utf16Slice, error) {
	if charCount == 0 {
		return Utf16Slice{}, nil
	}
	b, err := c.TakeBytes(charCount*2, "utf16_by_char_count")
	if err != nil {
		return Utf16Slice{}, err
	}
	return Utf16Slice{Bytes: b, Chars: effectiveChars(b)}, nil
}

func readLenPrefixedUtf16(c *bytesutil.Cursor, nullTerminated bool) (Utf16Slice, error) {
	charCount, err := c.U16("utf16_len_prefix")
	if err != nil {
		return Utf16Slice{}, err
	}
	s, err := readUtf16ByCharCount(c, int(charCount))
	if err != nil {
		return Utf16Slice{}, err
	}
	if nullTerminated {
		if _, err := c.U16("utf16_nul_terminator"); err != nil {
			return Utf16Slice{}, err
		}
	}
	return s, nil
}

func readNullTerminatedUtf16(c *bytesutil.Cursor) (Utf16Slice, error) {
	start := c.Pos()
	for {
		u, err := c.U16("null_terminated_utf16_string")
		if err != nil {
			return Utf16Slice{}, err
		}
		if u == 0 {
			end := c.Pos() - 2
			b, err := c.PeekAt(start, end-start, "null_terminated_utf16_string")
			if err != nil {
				return Utf16Slice{}, err
			}
			return Utf16Slice{Bytes: b, Chars: (end - start) / 2}, nil
		}
	}
}
