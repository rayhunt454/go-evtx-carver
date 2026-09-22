package carve

import (
	"bytes"
	"fmt"
	"os"

	"github.com/rayhunt454/go-evtx-carver/evtx"
	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
	"github.com/rayhunt454/go-evtx-carver/internal/render"
)

var chunkMagic = []byte("ElfChnk\x00")

// CarveChunks сканирует образ по imagePath на сигнатуры chunk'ов,
// валидирует каждого кандидата и декодирует из найденных chunk'ов все
// записи, какие получится.

// Буферизует все найденные записи в памяти — на большом образе с большим
// количеством совпадений это может занять непропорционально много памяти

func CarveChunks(imagePath string) ([]CarvedRecord, error) {
	records, errCh, err := CarveChunksStream(imagePath)
	if err != nil {
		return nil, err
	}
	return drainStream(records, errCh)
}

// CarveChunksStream — как CarveChunks, но отдаёт каждую найденную запись в
// канал records сразу по мере обнаружения, не накапливая их в памяти.
func CarveChunksStream(imagePath string) (<-chan CarvedRecord, <-chan error, error) {
	return carveStream(imagePath, func(f *os.File, size int64, out chan<- CarvedRecord) error {
		return findMagic(f, size, chunkMagic, func(offset int64) error {
			recs, cerr := carveChunkAt(f, size, offset)
			if cerr != nil {
				// Совпадение magic, не разобравшееся как заголовок chunk'а, не
				// прерывает сканирование — пропускаем кандидата и продолжаем.
				return nil
			}
			for _, rec := range recs {
				out <- rec
			}
			return nil
		})
	})
}

// carveChunkAt читает и валидирует chunk-кандидата, начинающегося с
// chunkOffset, и извлекает из него все записи.
func carveChunkAt(f *os.File, imageSize, chunkOffset int64) ([]CarvedRecord, error) {
	available := int64(evtx.ChunkSize)
	if chunkOffset+available > imageSize {
		available = imageSize - chunkOffset
	}
	if available < evtx.ChunkHeaderSize {
		return nil, fmt.Errorf("carve: only %d bytes available at offset %d, need at least the %d-byte chunk header", available, chunkOffset, evtx.ChunkHeaderSize)
	}

	buf := make([]byte, available)
	if _, err := f.ReadAt(buf, chunkOffset); err != nil {
		return nil, fmt.Errorf("carve: reading candidate chunk at offset %d: %w", chunkOffset, err)
	}

	chunk, err := evtx.NewChunkData(buf, false)
	if err != nil {
		return nil, fmt.Errorf("carve: parsing chunk header at offset %d: %w", chunkOffset, err)
	}

	confidence := ConfidenceChunkUnverified
	if chunk.ValidateHeaderChecksum() {
		if chunk.ValidateDataChecksum() {
			confidence = ConfidenceChunkValidated
		} else {
			confidence = ConfidenceChunkHeaderOnly
		}
	}

	return carveRecordsInChunk(chunk, chunkOffset, confidence), nil
}

// carveRecordsInChunk проходит область записей chunk.Data от
// ChunkHeaderSize до FreeSpaceOffset, извлекая каждую запись с валидным
// заголовком и DataSize, подтверждённым хвостовой копией.
func carveRecordsInChunk(chunk *evtx.ChunkData, chunkOffset int64, confidence Confidence) []CarvedRecord {
	data := chunk.Data
	freeSpace := int(chunk.Header.FreeSpaceOffset)
	if freeSpace > len(data) || freeSpace < evtx.ChunkHeaderSize {
		freeSpace = len(data)
	}

	var out []CarvedRecord
	var ctx *binxml.ChunkContext

	pos := evtx.ChunkHeaderSize
	for pos+evtx.RecordHeaderSize <= freeSpace {
		hdr, herr := evtx.ParseRecordHeaderAt(data, pos)
		if herr != nil {
			next, found := nextRecordMagic(data, pos+1, freeSpace)
			if !found {
				break
			}
			pos = next
			continue
		}

		recEnd := pos + int(hdr.DataSize)
		trailerOK := hdr.DataSize >= evtx.RecordHeaderSize+4 && recEnd <= len(data)
		if trailerOK {
			trailerCopy, terr := bytesutil.ReadU32LE(data, recEnd-4, "record_trailer_datasize")
			trailerOK = terr == nil && trailerCopy == hdr.DataSize
		}
		if !trailerOK {
			// Заголовок разобрался, но заявленный размер не совпал с
			// хвостовой копией
			next, found := nextRecordMagic(data, pos+1, freeSpace)
			if !found {
				break
			}
			pos = next
			continue
		}

		rec := CarvedRecord{
			ImageOffset:            chunkOffset + int64(pos),
			ChunkOffset:            chunkOffset,
			EventRecordID:          hdr.EventRecordID,
			Timestamp:              hdr.Timestamp,
			Confidence:             confidence,
			CompressedSourceOffset: -1,
		}

		binxmlOffset := pos + evtx.RecordHeaderSize
		binxmlSize := int(hdr.DataSize) - evtx.RecordHeaderSize - 4
		if binxmlSize < 0 {
			binxmlSize = 0
		}

		if ctx == nil {
			ctx = binxml.NewChunkContext(data, nil)
		}
		root, perr := ctx.ParseRecord(binxmlOffset, binxmlSize)
		if perr != nil {
			rec.Note = fmt.Sprintf("structure unavailable: %v", perr)
		} else if line, jerr := render.JSON(root); jerr != nil {
			rec.Note = fmt.Sprintf("rendering JSON: %v", jerr)
		} else {
			rec.JSON = line
		}

		out = append(out, rec)
		pos += int(hdr.DataSize)
	}

	return out
}

func nextRecordMagic(data []byte, from, limit int) (int, bool) {
	if from >= limit || from < 0 {
		return 0, false
	}
	idx := bytes.Index(data[from:limit], recordMagic)
	if idx < 0 {
		return 0, false
	}
	return from + idx, true
}
