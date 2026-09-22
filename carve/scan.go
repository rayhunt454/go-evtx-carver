package carve

import (
	"bytes"
	"fmt"
	"io"
)

const scanWindowSize = 4 << 20 // 4 MiB

func findMagic(ra io.ReaderAt, size int64, magic []byte, onMatch func(offset int64) error) error {
	if size < int64(len(magic)) {
		return nil
	}
	overlap := len(magic) - 1
	stride := int64(scanWindowSize - overlap)
	buf := make([]byte, scanWindowSize)

	for base := int64(0); base < size; base += stride {
		want := int64(scanWindowSize)
		if base+want > size {
			want = size - base
		}
		n, err := ra.ReadAt(buf[:want], base)
		if err != nil && err != io.EOF {
			return fmt.Errorf("carve: reading image at offset %d: %w", base, err)
		}
		data := buf[:n]

		isLastWindow := base+int64(n) >= size
		searchLimit := n
		if !isLastWindow {

			searchLimit = n - overlap
		}

		upper := searchLimit + overlap
		if upper > n {
			upper = n
		}
		for i := 0; i < searchLimit; {
			idx := bytes.Index(data[i:upper], magic)
			if idx < 0 {
				break
			}
			if err := onMatch(base + int64(i+idx)); err != nil {
				return err
			}
			i += idx + 1
		}

		if isLastWindow {
			break
		}
	}
	return nil
}
