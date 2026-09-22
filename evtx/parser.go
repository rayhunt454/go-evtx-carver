package evtx

import (
	"fmt"
	"io"
	"os"
)

// ReadSeeker — минимальный интерфейс, нужный Parser от источника данных:
// os.File ему удовлетворяет напрямую, а буфер в памяти можно обернуть через
// bytes.NewReader.
type ReadSeeker interface {
	io.Reader
	io.Seeker
}

// Parser читает заголовок EVTX-файла один раз при создании и затем даёт
// потоковый доступ к его чанкам и записям. В памяти держит только текущий
// чанк (65536 байт), а не весь файл.
type Parser struct {
	r        ReadSeeker
	closer   io.Closer // задан, если Parser владеет r (например, через Open)
	Header   *FileHeader
	Settings ParserSettings

	// calculatedChunkCount = (размер потока - размер блока заголовка) / ChunkSize.
	// Поле ChunkCount самого заголовка файла — uint16 и ненадёжно для больших
	// файлов, поэтому итерация чанков опирается на это значение.
	calculatedChunkCount uint64
}

// Open открывает EVTX-файл по пути path и разбирает его заголовок.
// Возвращённый Parser владеет файлом и закроет его при вызове Close.
func Open(path string) (*Parser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	p, err := NewParser(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	p.closer = f
	return p, nil
}

// NewParser читает и разбирает заголовок EVTX-файла из r, вычисляя число
// чанков по общей длине потока. Владение r остаётся за вызывающим кодом;
// используйте Open, если нужно, чтобы Parser сам владел файлом (и закрывал
// его).
func NewParser(r ReadSeeker) (*Parser, error) {
	headerBlock := make([]byte, FileHeaderBlockSize)
	if _, err := io.ReadFull(r, headerBlock); err != nil {
		return nil, fmt.Errorf("failed to read EVTX file header block: %w", err)
	}

	header, err := ParseFileHeader(headerBlock)
	if err != nil {
		return nil, err
	}

	streamSize, err := streamLen(r)
	if err != nil {
		return nil, err
	}
	if streamSize < uint64(header.HeaderBlockSize) {
		return nil, fmt.Errorf("could not calculate a valid chunk count: stream size (%d) is less than the header block size (%d)", streamSize, header.HeaderBlockSize)
	}
	chunkDataSize := streamSize - uint64(header.HeaderBlockSize)

	return &Parser{
		r:                    r,
		Header:               header,
		Settings:             DefaultSettings(),
		calculatedChunkCount: chunkDataSize / ChunkSize,
	}, nil
}

// ChunkCount возвращает число чанков, которое парсер попытается прочитать,
// вычисленное по общей длине потока.
func (p *Parser) ChunkCount() uint64 { return p.calculatedChunkCount }

// Close закрывает файл, если Parser был создан через Open; иначе — no-op.
func (p *Parser) Close() error {
	if p.closer != nil {
		return p.closer.Close()
	}
	return nil
}

// streamLen возвращает общую длину r, не нарушая его текущую позицию.
func streamLen(r ReadSeeker) (uint64, error) {
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if cur != end {
		if _, err := r.Seek(cur, io.SeekStart); err != nil {
			return 0, err
		}
	}
	return uint64(end), nil
}

// allZero сообщает, состоит ли b целиком из нулевых байт.
func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// allocateChunk читает и разбирает чанк по смещению, ожидаемому для
// chunkNumber. Возвращает (nil, nil), если чанк целиком заполнен нулями
// (дыра в незавершённом/разреженном файле — вызывающему стоит попробовать
// следующий номер чанка). Ненулевая ошибка означает проблему при чтении или
// разборе; если при этом чанк всё же вернулся не пустым, он пригоден для
// использования (см. NewChunkData) — это не обязательно повод его отбросить.
func (p *Parser) allocateChunk(chunkNumber uint64) (*ChunkData, error) {
	offset := int64(FileHeaderBlockSize) + int64(chunkNumber)*int64(ChunkSize)
	if _, err := p.r.Seek(offset, io.SeekStart); err != nil {
		return nil, &ChunkParseError{ChunkID: chunkNumber, Err: err}
	}

	buf := make([]byte, ChunkSize)
	if _, err := io.ReadFull(p.r, buf); err != nil {
		return nil, &ChunkParseError{ChunkID: chunkNumber, Err: ErrIncompleteChunk}
	}

	if allZero(buf) {
		return nil, nil
	}

	chunk, err := NewChunkData(buf, p.Settings.ValidateChecksums)
	if err != nil {
		return chunk, &ChunkParseError{ChunkID: chunkNumber, Err: err}
	}
	return chunk, nil
}

// findNextChunk ищет следующий непустой чанк начиная с chunkNumber
// (включительно). found равен false, когда поиск ушёл за calculatedChunkCount
// безрезультатно — итерация завершена.
func (p *Parser) findNextChunk(chunkNumber uint64) (chunk *ChunkData, resultChunkNumber uint64, err error, found bool) {
	for {
		c, allocErr := p.allocateChunk(chunkNumber)
		if allocErr != nil {
			if chunkNumber >= p.calculatedChunkCount {
				return nil, 0, nil, false
			}
			return c, chunkNumber, allocErr, true
		}
		if c == nil {
			// Пустой (нулевой) чанк: продолжаем поиск, но тоже завершаем
			// итерацию, если ушли за расчётное число чанков.
			if chunkNumber >= p.calculatedChunkCount {
				return nil, 0, nil, false
			}
			chunkNumber++
			continue
		}
		return c, chunkNumber, nil, true
	}
}

// ChunkIterator последовательно перебирает все чанки файла Parser.
type ChunkIterator struct {
	p       *Parser
	current uint64
}

// Chunks возвращает итератор по всем чанкам файла.
func (p *Parser) Chunks() *ChunkIterator {
	return &ChunkIterator{p: p}
}

// Next возвращает следующий чанк или (nil, nil), когда итерация завершена.
// Ненулевая ошибка означает проблему с текущим чанком; итерация продолжится
// со следующего чанка при следующем вызове. Если при этом chunk не nil (это
// случай несовпадения контрольной суммы, см. NewChunkData), он пригоден для
// разбора — отбрасывать его не обязательно.
func (it *ChunkIterator) Next() (*ChunkData, error) {
	chunk, chunkNumber, err, found := it.p.findNextChunk(it.current)
	if !found {
		return nil, nil
	}
	it.current = chunkNumber + 1
	return chunk, err
}

// RecordsIterator последовательно перебирает все записи всех чанков файла
// Parser, прозрачно переходя от одного чанка к следующему.
type RecordsIterator struct {
	chunks  *ChunkIterator
	current *RecordIterator
}

// Records возвращает итератор по всем записям файла (чанки обходятся
// последовательно, в один поток).
func (p *Parser) Records() *RecordsIterator {
	return &RecordsIterator{chunks: p.Chunks()}
}

// Next возвращает следующую запись или (nil, nil), когда файл полностью
// пройден. Ненулевая ошибка означает, что чанк или запись не разобрались;
// итерация продолжится со следующего чанка при последующих вызовах. Чанк с
// несовпавшей контрольной суммой не пропускается — его записи всё равно
// читаются, ошибка лишь сообщается один раз.
func (it *RecordsIterator) Next() (*Record, error) {
	for {
		if it.current != nil {
			rec, err := it.current.Next()
			if err != nil {
				return nil, err
			}
			if rec != nil {
				return rec, nil
			}
			// Этот чанк исчерпан; переходим к следующему.
			it.current = nil
		}

		chunk, err := it.chunks.Next()
		if chunk != nil {
			it.current = chunk.Records()
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			return nil, nil
		}
	}
}
