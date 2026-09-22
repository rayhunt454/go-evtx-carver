package carve

import (
	"fmt"
	"os"

	"github.com/rayhunt454/go-evtx-carver/evtx"
	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// recordMagic — 4-байтовая сигнатура начала каждой EVTX-записи.
var recordMagic = []byte{0x2a, 0x2a, 0x00, 0x00}

// CarveRecordsByMagic сканирует образ по imagePath напрямую на сигнатуры
// отдельных записей (0x2A 0x2A 0x00 0x00), независимо от владеющего chunk'а.
//
// Имена полей не восстанавливаются — они всегда ссылаются на кэш строк
// chunk'а, которого здесь нет. Вместо этого декодируется всё, что
// самодостаточно на уровне записи: EventRecordID и Timestamp из заголовка,
// и — если корень BinXML является TemplateInstance — все значения
// подстановок позиционно, в Values. Если декодирование как TemplateInstance
// невозможно, используется резервный сырой sweep текста в RawStrings.

func CarveRecordsByMagic(imagePath string) ([]CarvedRecord, error) {
	records, errCh, err := CarveRecordsByMagicStream(imagePath)
	if err != nil {
		return nil, err
	}
	return drainStream(records, errCh)
}

// CarveRecordsByMagicStream — потоковый вариант CarveRecordsByMagic: отдаёт
// каждую запись в канал records по мере обнаружения
func CarveRecordsByMagicStream(imagePath string) (<-chan CarvedRecord, <-chan error, error) {
	return carveStream(imagePath, func(f *os.File, size int64, out chan<- CarvedRecord) error {
		return findMagic(f, size, recordMagic, func(offset int64) error {
			rec, ok := carveRecordEnvelopeAt(f, size, offset)
			if ok {
				out <- rec
			}
			return nil
		})
	})
}

// carveRecordEnvelopeAt проверяет заголовок записи-кандидата по offset и,
// если он самосогласован (DataSize подтверждён хвостовой копией), извлекает
// его содержимое.
func carveRecordEnvelopeAt(f *os.File, imageSize, offset int64) (CarvedRecord, bool) {
	headerBuf := make([]byte, evtx.RecordHeaderSize)
	if _, err := f.ReadAt(headerBuf, offset); err != nil {
		return CarvedRecord{}, false
	}
	hdr, err := evtx.ParseRecordHeaderAt(headerBuf, 0)
	if err != nil {
		return CarvedRecord{}, false
	}
	if hdr.DataSize < evtx.RecordHeaderSize+4 {
		return CarvedRecord{}, false
	}
	if offset+int64(hdr.DataSize) > imageSize {
		return CarvedRecord{}, false
	}

	trailerCopy, err := readU32At(f, offset+int64(hdr.DataSize)-4)
	if err != nil || trailerCopy != hdr.DataSize {

		return CarvedRecord{}, false
	}

	rec := CarvedRecord{
		ImageOffset:            offset,
		ChunkOffset:            -1,
		EventRecordID:          hdr.EventRecordID,
		Timestamp:              hdr.Timestamp,
		Confidence:             ConfidenceRecordEnvelope,
		CompressedSourceOffset: -1,
	}

	full := make([]byte, hdr.DataSize)
	if _, err := f.ReadAt(full, offset); err != nil {
		rec.Note = fmt.Sprintf("read envelope only: could not re-read full record body: %v", err)
		return rec, true
	}
	binxmlSpan := full[evtx.RecordHeaderSize : len(full)-4]

	if ti, err := binxml.ParseStandaloneTemplateInstance(binxmlSpan, winenc.DecodeWindows1252); err == nil {
		rec.Values = formatValues(ti.Values, winenc.DecodeWindows1252)
		rec.ValueTypes = valueTypeNames(ti.Values)
		rec.Note = "template instance decoded without a chunk: field names are not recoverable (chunk-relative), but every substitution value is listed positionally in Values (with its wire type in ValueTypes), in declaration order"
		return rec, true
	}

	rec.RawStrings = sweepUTF16Strings(binxmlSpan, 4)
	if len(rec.RawStrings) == 0 {
		rec.Note = "not decodable as a standalone template instance, and no text found in raw-string sweep either"
	} else {
		rec.Note = "not decodable as a standalone template instance; see RawStrings for whatever text was recovered by sweeping raw bytes"
	}
	return rec, true
}

// readU32At читает little-endian uint32 по абсолютному смещению в f
func readU32At(f *os.File, offset int64) (uint32, error) {
	buf := make([]byte, 4)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return 0, err
	}
	return bytesutil.ReadU32LE(buf, 0, "trailer_datasize")
}

// sweepUTF16Strings ищет в data участки ASCII-совместимых UTF-16LE символов
// длиной не менее minChars и возвращает их как строки Go.
func sweepUTF16Strings(data []byte, minChars int) []string {
	var out []string
	var run []byte
	flush := func() {
		if len(run) >= minChars {
			out = append(out, string(run))
		}
		run = run[:0]
	}
	for i := 0; i+1 < len(data); i += 2 {
		lo, hi := data[i], data[i+1]
		if hi == 0 && lo >= 0x20 && lo < 0x7f {
			run = append(run, lo)
			continue
		}
		flush()
	}
	flush()
	return out
}
