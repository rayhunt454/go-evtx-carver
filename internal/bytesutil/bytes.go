// Пакет bytesutil предоставляет средства чтения с проверкой границ в формате little-endian для
// байтовых фрагментов в памяти.

// EVTX (и вложенный в него формат BinXML) полностью использует формат little-endian.
package bytesutil

import "fmt"

// Ошибка TruncatedError возвращается, когда чтение выходит за пределы буфера, из которого производится чтение.
type TruncatedError struct {
	What   string // удобочитаемое описание того, что читалось
	Offset int    // смещение, с которого началось чтение,
	Need   int    // количество необходимых байтов
	Have   int    // количество байтов, фактически доступных из смещения
}

func (e *TruncatedError) Error() string {
	return fmt.Sprintf("buffer too small for %s at offset %d (need %d bytes, have %d)", e.What, e.Offset, e.Need, e.Have)
}

func truncated(what string, offset, need, bufLen int) *TruncatedError {
	have := bufLen - offset
	if have < 0 {
		have = 0
	}
	return &TruncatedError{What: what, Offset: offset, Need: need, Have: have}
}

// Функция Slice возвращает buf[offset : offset+length] или *TruncatedError, если этот диапазон не помещается внутри buf.
func Slice(buf []byte, offset, length int, what string) ([]byte, error) {
	if offset < 0 || length < 0 || offset+length > len(buf) {
		return nil, truncated(what, offset, length, len(buf))
	}
	return buf[offset : offset+length], nil
}

func ReadU8(buf []byte, offset int, what string) (uint8, error) {
	b, err := Slice(buf, offset, 1, what)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func ReadU16LE(buf []byte, offset int, what string) (uint16, error) {
	b, err := Slice(buf, offset, 2, what)
	if err != nil {
		return 0, err
	}
	return uint16(b[0]) | uint16(b[1])<<8, nil
}

func ReadU32LE(buf []byte, offset int, what string) (uint32, error) {
	b, err := Slice(buf, offset, 4, what)
	if err != nil {
		return 0, err
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24, nil
}

func ReadU64LE(buf []byte, offset int, what string) (uint64, error) {
	b, err := Slice(buf, offset, 8, what)
	if err != nil {
		return 0, err
	}
	var v uint64
	for i := 7; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v, nil
}
