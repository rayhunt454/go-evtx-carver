package evtx

// ChunkData — полностью прочитанный чанк EVTX размером ChunkSize: разобранный
// заголовок плюс сырые байты (записи вырезаются из Data итератором записей
// без копирования).
type ChunkData struct {
	Header *ChunkHeader
	Data   []byte
}

// NewChunkData разбирает data (ровно ChunkSize байт) как чанк EVTX. Магия
// заголовка проверяется всегда; контрольные суммы CRC32 — только если
// validateChecksum true. При их несовпадении возвращается ненулевая
// ErrInvalidChunkChecksum, но chunk всё равно возвращается — данные в data
// физически присутствуют и разбираемы, несовпадение контрольной суммы лишь
// означает, что они не гарантированно целы.
func NewChunkData(data []byte, validateChecksum bool) (*ChunkData, error) {
	header, err := ParseChunkHeader(data)
	if err != nil {
		return nil, err
	}

	chunk := &ChunkData{Header: header, Data: data}
	if validateChecksum && !chunk.ValidateChecksum() {
		return chunk, ErrInvalidChunkChecksum
	}
	return chunk, nil
}

// ValidateHeaderChecksum сообщает, совпадает ли сохранённая в заголовке
// чанка контрольная сумма с вычисленной CRC32-IEEE суммой байт заголовка
// (data[0:120] ++ data[128:512]). При установленном флаге NO_CRC32 проверка
// тривиально проходит.
func (c *ChunkData) ValidateHeaderChecksum() bool {
	if c.Header.Flags.Has(ChunkFlagNoCRC32) {
		return true
	}
	headerBytes := make([]byte, 0, 120+384)
	headerBytes = append(headerBytes, c.Data[:120]...)
	headerBytes = append(headerBytes, c.Data[128:512]...)
	return crc32IEEE(headerBytes) == c.Header.HeaderChunkChecksum
}

// ValidateDataChecksum сообщает, совпадает ли сохранённая контрольная сумма
// событий с вычисленной CRC32-IEEE суммой данных записей
// (data[512:FreeSpaceOffset]). FreeSpaceOffset — непроверенное значение из
// файла, поэтому предварительно ограничивается длиной чанка.
func (c *ChunkData) ValidateDataChecksum() bool {
	if c.Header.Flags.Has(ChunkFlagNoCRC32) {
		return true
	}
	end := int(c.Header.FreeSpaceOffset)
	if end > len(c.Data) {
		end = len(c.Data)
	}
	if end < ChunkHeaderSize {
		end = ChunkHeaderSize
	}
	return crc32IEEE(c.Data[ChunkHeaderSize:end]) == c.Header.EventsChecksum
}

// ValidateChecksum сообщает, верны ли обе контрольные суммы — заголовка и
// данных.
func (c *ChunkData) ValidateChecksum() bool {
	return c.ValidateHeaderChecksum() && c.ValidateDataChecksum()
}

// Records возвращает итератор по записям чанка (только заголовок и сырой
// BinXML, без его разбора).
func (c *ChunkData) Records() *RecordIterator {
	return &RecordIterator{chunk: c, offset: ChunkHeaderSize}
}
