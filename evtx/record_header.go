package evtx

import (
	"time"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// RecordHeaderSize — это размер структурированного заголовка записи.
// Сразу за ним следует полезная нагрузка записи в формате BinXML.
const RecordHeaderSize = 24

// recordHeaderMagic — это 4-байтовая сигнатура, с которой начинается каждая запись EVTX.
var recordHeaderMagic = [4]byte{0x2a, 0x2a, 0x00, 0x00}

// RecordHeader — это фиксированный структурированный заголовок размером 24 байта,
// расположенный в начале каждой записи EVTX.
type RecordHeader struct {
	DataSize      uint32
	EventRecordID uint64
	Timestamp     time.Time
}

// ParseRecordHeaderAt разбирает 24-байтовый заголовок записи EVTX,
// начинающийся со смещения offset в буфере buf.
func ParseRecordHeaderAt(buf []byte, offset int) (*RecordHeader, error) {
	if _, err := bytesutil.Slice(buf, offset, RecordHeaderSize, "EVTX record header"); err != nil {
		return nil, err
	}

	magicBytes, _ := bytesutil.Slice(buf, offset, 4, "record header magic")
	var magic [4]byte
	copy(magic[:], magicBytes)
	if magic != recordHeaderMagic {
		return nil, &RecordMagicError{Magic: magic}
	}

	dataSize, err := bytesutil.ReadU32LE(buf, offset+4, "record.data_size")
	if err != nil {
		return nil, err
	}
	eventRecordID, err := bytesutil.ReadU64LE(buf, offset+8, "record.event_record_id")
	if err != nil {
		return nil, err
	}
	filetime, err := bytesutil.ReadU64LE(buf, offset+16, "record.filetime")
	if err != nil {
		return nil, err
	}
	timestamp, err := winenc.FileTimeToTime(filetime)
	if err != nil {
		return nil, err
	}

	return &RecordHeader{
		DataSize:      dataSize,
		EventRecordID: eventRecordID,
		Timestamp:     timestamp,
	}, nil
}

// BinXMLDataSize возвращает длину (в байтах) полезной нагрузки BinXML
// данной записи: DataSize за вычетом 24-байтового заголовка и 4-байтовой
// копии значения DataSize в конце.
func (h *RecordHeader) BinXMLDataSize() (uint32, error) {
	const overhead = RecordHeaderSize + 4
	if h.DataSize < overhead {
		return 0, &InvalidDataSizeError{Length: h.DataSize, Expected: overhead}
	}
	return h.DataSize - overhead, nil
}
