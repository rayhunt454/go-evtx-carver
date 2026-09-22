package evtx

import "time"

// Запись представляет собой одну запись EVTX: её идентификатор (ID записи события, метка времени) и
// ссылку на необработанные, неразобранные данные в формате BinXML.
type Record struct {
	Header RecordHeader

	binxmlOffset int // offset of the BinXML payload within chunk.Data
	binxmlSize   uint32
	chunk        *ChunkData
}

// EventRecordID возвращает уникальный идентификатор записи.
func (r *Record) EventRecordID() uint64 { return r.Header.EventRecordID }

func (r *Record) Timestamp() time.Time { return r.Header.Timestamp }

// RawBinXML возвращает необработанные, неразобранные данные BinXML записи
func (r *Record) RawBinXML() []byte {
	return r.chunk.Data[r.binxmlOffset : r.binxmlOffset+int(r.binxmlSize)]
}

func (r *Record) ChunkData() []byte {
	return r.chunk.Data
}

func (r *Record) BinXMLOffset() int {
	return r.binxmlOffset
}

func (r *Record) BinXMLSize() int {
	return int(r.binxmlSize)
}
