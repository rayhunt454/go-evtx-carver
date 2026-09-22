package binxml

import "github.com/rayhunt454/go-evtx-carver/internal/bytesutil"

// stringCache  декодирует  имена строковых таблиц фрагментов по их
// относительному смещению в байтах фрагмента.
type stringCache struct {
	data []byte
	m    map[uint32]string
}

func newStringCache(data []byte) *stringCache {
	return &stringCache{data: data, m: make(map[uint32]string)}
}

// Функция resolve декодирует (и мемоизирует) запись имени по смещению относительно блока.
func (sc *stringCache) resolve(offset uint32) (string, error) {
	if s, ok := sc.m[offset]; ok {
		return s, nil
	}
	c, err := bytesutil.NewCursorAt(sc.data, int(offset))
	if err != nil {
		return "", err
	}
	if _, err := c.TakeBytes(6, "name_link"); err != nil {
		return "", err
	}
	name, err := readLenPrefixedUtf16(c, true)
	if err != nil {
		return "", err
	}
	s := name.String()
	sc.m[offset] = s
	return s, nil
}
