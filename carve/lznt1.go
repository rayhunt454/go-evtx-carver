package carve

import (
	"bytes"
	"fmt"
	"os"

	"github.com/rayhunt454/go-evtx-carver/evtx"
)

// lznt1.go восстанавливает EVTX chunk'и, которые физически лежат в образе не
// сырыми байтами, а внутри потока LZNT1 — классического посегментного сжатия
// NTFS (атрибут FILE_ATTRIBUTE_COMPRESSED).

// Единица сжатия NTFS (Compression Unit, CU) — 16 кластеров. При кластере в
// 4 КБ (подавляющее большинство реальных томов) это ровно 64 КБ — то есть
// ровно evtx.ChunkSize. А поскольку размер chunk'а EVTX фиксирован и все
// chunk'и идут в файле подряд без зазоров, границы chunk'ов в исходном файле
// всегда совпадают с границами CU. Из этого следует ключевой трюк: магия
// "ElfChnk\x00" (первые 8 байт chunk'а) всегда оказывается первыми байтами
// содержимого, а самые первые байты любого LZ77-потока обязаны быть
// литералами, потому что окну назад ссылаться ещё не на что. Значит магия
// почти всегда присутствует в самом потоке байт как есть, просто с
// небольшим фиксированным сдвигом: 2 байта заголовка LZNT1-подчанка (+1 байт
// флагов, если этот подчанк сам помечен сжатым) отделяют начало CU от начала
// магии.
//
// Отсюда алгоритм: искать "ElfChnk\x00" обычным сканом (та же сигнатура и
// тот же findMagic, что и в CarveChunks), а на каждой находке пробовать
// отступить на 3 байта назад (первый подчанк CU сам сжат) или на 2 байта
// (первый подчанк CU хранится как есть) и раскодировать оттуда LZNT1 до
// набора ровно evtx.ChunkSize байт. Получившийся буфер — это обычный,
// несжатый chunk EVTX, который дальше проверяется и разбирается той же
// логикой, что и в CarveChunks (заголовок, чексуммы, обход записей) — здесь
// нет отдельной, параллельной ветки разбора.

const (
	lznt1SubchunkMaxSize = 4096

	lznt1SearchWindow = evtx.ChunkSize + 16*2 + 256
)

// lznt1CandidateBackoffs — на сколько байт отступить назад от найденной
// магии chunk'а
var lznt1CandidateBackoffs = [2]int64{3, 2}

var (
	errLZNT1Truncated = fmt.Errorf("lznt1: unexpected end of data")
	errLZNT1BadHeader = fmt.Errorf("lznt1: invalid subchunk header signature")
	errLZNT1BadOffset = fmt.Errorf("lznt1: back-reference points before start of subchunk")
	errLZNT1Overflow  = fmt.Errorf("lznt1: decoded data exceeds expected size")
)

// lznt1Header — разобранный 2-байтный заголовок LZNT1-подчанка.
type lznt1Header struct {
	compressed bool
	payloadLen int
}

func parseLZNT1Header(h uint16) (lznt1Header, bool) {
	if h&0x7000 != 0x3000 {
		return lznt1Header{}, false
	}
	return lznt1Header{
		compressed: h&0x8000 != 0,
		payloadLen: int(h&0x0FFF) + 1,
	}, true
}

// lznt1DecompressSubchunk раскодирует один сжатый подчанк (payload — данные
// подчанка без 2-байтного заголовка) в out (переиспользуемый буфер длиной
// lznt1SubchunkMaxSize). Возвращает число раскодированных байт.

func lznt1DecompressSubchunk(payload []byte, out []byte) (int, error) {
	pos := 0
	pow2 := 0x10
	mask := uint16(0x0FFF)
	shift := uint(12)

	i := 0
	for i < len(payload) {
		flags := payload[i]
		i++
		for bit := 0; bit < 8 && i < len(payload); bit++ {
			if flags&1 == 0 {
				// Литерал: один байт как есть.
				if pos >= len(out) {
					return 0, errLZNT1Overflow
				}
				out[pos] = payload[i]
				i++
				pos++
			} else {
				// Символ "смещение назад/длина" — сначала подгоняем ширину
				// полей под текущую позицию pos внутри подчанка.
				for pos > pow2 {
					pow2 <<= 1
					mask >>= 1
					shift--
				}
				if i+2 > len(payload) {
					return 0, errLZNT1Truncated
				}
				sym := uint16(payload[i]) | uint16(payload[i+1])<<8
				i += 2
				length := int(sym&mask) + 3
				offset := int(sym>>shift) + 1
				if offset > pos {
					return 0, errLZNT1BadOffset
				}
				if pos+length > len(out) {
					return 0, errLZNT1Overflow
				}

				for k := 0; k < length; k++ {
					out[pos] = out[pos-offset]
					pos++
				}
			}
			flags >>= 1
		}
	}
	return pos, nil
}

// lznt1DecodeUnit декодирует Compression Unit, начинающийся в src[0:],
// последовательно читая подчанки, пока не наберёт ровно maxOut
// раскодированных байт (для chunk'а EVTX — evtx.ChunkSize).
func lznt1DecodeUnit(src []byte, maxOut int) (out []byte, consumed int, err error) {
	out = make([]byte, 0, maxOut)
	subOut := make([]byte, lznt1SubchunkMaxSize)
	srcPos := 0

	for len(out) < maxOut {
		if srcPos+2 > len(src) {
			return nil, 0, errLZNT1Truncated
		}
		h := uint16(src[srcPos]) | uint16(src[srcPos+1])<<8
		if h == 0 {
			srcPos += 2
			out = append(out, make([]byte, maxOut-len(out))...)
			return out, srcPos, nil
		}
		hdr, ok := parseLZNT1Header(h)
		if !ok {
			return nil, 0, errLZNT1BadHeader
		}
		payloadStart := srcPos + 2
		payloadEnd := payloadStart + hdr.payloadLen
		if payloadEnd > len(src) {
			return nil, 0, errLZNT1Truncated
		}
		payload := src[payloadStart:payloadEnd]

		var n int
		if hdr.compressed {
			n, err = lznt1DecompressSubchunk(payload, subOut)
			if err != nil {
				return nil, 0, err
			}
		} else {
			if len(payload) > lznt1SubchunkMaxSize {
				return nil, 0, errLZNT1BadHeader
			}
			n = copy(subOut, payload)
		}
		if len(out)+n > maxOut {
			return nil, 0, errLZNT1Overflow
		}
		out = append(out, subOut[:n]...)
		srcPos = payloadEnd
	}
	return out, srcPos, nil
}

// CarveChunksLZNT1 ищет chunk'и EVTX, сжатые NTFS-компрессией (LZNT1) — см.
// комментарий в начале файла про сам трюк. Как и CarveChunks, использует
// сигнатуру "ElfChnk\x00", но интерпретирует байты вокруг находки не как сам
// chunk, а как содержимое, зашифрованное LZNT1-подчанками, из которого
// нужно ещё восстановить исходные evtx.ChunkSize байт.

func CarveChunksLZNT1(imagePath string) ([]CarvedRecord, error) {
	records, errCh, err := CarveChunksLZNT1Stream(imagePath)
	if err != nil {
		return nil, err
	}
	return drainStream(records, errCh)
}

// CarveChunksLZNT1Stream — потоковый вариант CarveChunksLZNT1: отдаёт
// каждую запись в канал records по мере обнаружения, не накапливая их в
// памяти.
func CarveChunksLZNT1Stream(imagePath string) (<-chan CarvedRecord, <-chan error, error) {
	return carveStream(imagePath, func(f *os.File, size int64, out chan<- CarvedRecord) error {
		return findMagic(f, size, chunkMagic, func(magicOffset int64) error {
			recs, ok := carveChunkAtLZNT1(f, size, magicOffset)
			if ok {
				for _, rec := range recs {
					out <- rec
				}
			}
			return nil
		})
	})
}

// carveChunkAtLZNT1 пробует оба варианта отступа (lznt1CandidateBackoffs) от
// magicOffset, раскодирует LZNT1 до полного chunk'а и, если магия на месте
// (0..7 байт результата), проверяет и разбирает его той же логикой, что и
// carveChunkAt — обычный сырой путь.
func carveChunkAtLZNT1(f *os.File, imageSize, magicOffset int64) ([]CarvedRecord, bool) {
	for _, backoff := range lznt1CandidateBackoffs {
		candStart := magicOffset - backoff
		if candStart < 0 {
			continue
		}
		want := int64(lznt1SearchWindow)
		if candStart+want > imageSize {
			want = imageSize - candStart
		}
		if want < 2 {
			continue
		}

		src := make([]byte, want)
		if _, err := f.ReadAt(src, candStart); err != nil {
			continue
		}

		chunkBuf, consumed, err := lznt1DecodeUnit(src, evtx.ChunkSize)
		if err != nil || !bytes.Equal(chunkBuf[:len(chunkMagic)], chunkMagic) {
			continue
		}

		chunk, err := evtx.NewChunkData(chunkBuf, false)
		if err != nil {
			continue
		}

		confidence := ConfidenceChunkUnverified
		if chunk.ValidateHeaderChecksum() {
			if chunk.ValidateDataChecksum() {
				confidence = ConfidenceChunkValidated
			} else {
				confidence = ConfidenceChunkHeaderOnly
			}
		}

		recs := carveRecordsInChunk(chunk, candStart, confidence)
		for i := range recs {
			recs[i].CompressedSourceOffset = candStart
			recs[i].CompressedSourceBytes = consumed
			recs[i].Note = withLZNT1Note(recs[i].Note, candStart, consumed)
		}
		return recs, true
	}
	return nil, false
}

func withLZNT1Note(note string, candStart int64, consumed int) string {
	add := fmt.Sprintf("recovered by decoding LZNT1 (NTFS compression) starting at image offset %d (%d compressed bytes -> %d bytes)", candStart, consumed, evtx.ChunkSize)
	if note == "" {
		return add
	}
	return note + "; " + add
}
