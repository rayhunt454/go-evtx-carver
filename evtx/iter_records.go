package evtx

import "fmt"

// RecordIterator перебирает записи одного чанка, используя смещене (record_start += record.DataSize), и останавливается
// на значении FreeSpaceOffset этого чанка. Он не выполняет разбор
// содержимого BinXML.
//
// Экземпляр RecordIterator можно получить из (*ChunkData).Records.
type RecordIterator struct {
	chunk     *ChunkData
	offset    int
	exhausted bool
}

// Метод Next возвращает следующую запись из чанка. Он возвращает (nil, nil)
// по завершении итерации (независимо от того, закончился ли чанк штатно или
// была встречена поврежденная запись, воспринятая как «мягкий» конец чанка —
// см. RecordMagicError.IsZero). Ошибка, отличная от nil, означает реальный
// сбой при разборе; после этого итерация прекращается (самый следующий вызов
// вернет (nil, nil)), что соответствует поведению эталонной реализации,
// где одна некорректная запись приводит к прерыванию обработки только
// оставшейся части текущего чанка, а не всего файла.
func (it *RecordIterator) Next() (*Record, error) {
	if it.exhausted {
		return nil, nil
	}

	data := it.chunk.Data

	// FreeSpaceOffset — это ненадежные данные, определяемые содержимым файла;
	// в поврежденном файле они могут указывать за пределы чанка,
	// поэтому значение следует ограничить в целях безопасности.
	freeSpaceOffset := int(it.chunk.Header.FreeSpaceOffset)
	if freeSpaceOffset > len(data) {
		freeSpaceOffset = len(data)
	}

	if it.offset >= freeSpaceOffset {
		it.exhausted = true
		return nil, nil
	}
	if len(data)-it.offset < RecordHeaderSize {

		it.exhausted = true
		return nil, nil
	}

	header, err := ParseRecordHeaderAt(data, it.offset)
	if err != nil {
		it.exhausted = true

		if magicErr, ok := err.(*RecordMagicError); ok && magicErr.IsZero() {

			return nil, nil
		}
		return nil, err
	}

	binxmlSize, err := header.BinXMLDataSize()
	if err != nil {
		it.exhausted = true
		return nil, err
	}

	binxmlStart := it.offset + RecordHeaderSize
	binxmlEnd := binxmlStart + int(binxmlSize)
	if binxmlEnd > len(data) {
		it.exhausted = true
		return nil, &RecordParseError{
			RecordID: header.EventRecordID,
			Err:      fmt.Errorf("record BinXML slice [%d:%d) is out of bounds (chunk data length %d)", binxmlStart, binxmlEnd, len(data)),
		}
	}

	rec := &Record{
		Header:       *header,
		binxmlOffset: binxmlStart,
		binxmlSize:   binxmlSize,
		chunk:        it.chunk,
	}

	it.offset += int(header.DataSize)
	if it.chunk.Header.LastEventRecordID == header.EventRecordID {
		it.exhausted = true
	}

	return rec, nil
}
