package evtx

import (
	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

// fileHeaderMagic — это 8-байтовая сигнатура (магическое число), с которой начинается любой файл EVTX.
const fileHeaderMagic = "ElfFile\x00"

// FileHeaderBlockSize — это количество байтов, зарезервированных на диске
// для блока заголовка файла. Структурированы только первые 128 байтов;
// остальное — заполнитель. Первый фрагмент данных начинается сразу
// после этого блока.
const FileHeaderBlockSize = 4096

type HeaderFlags uint32

const (
	HeaderFlagEmpty   HeaderFlags = 0x0
	HeaderFlagDirty   HeaderFlags = 0x1
	HeaderFlagFull    HeaderFlags = 0x2
	HeaderFlagNoCRC32 HeaderFlags = 0x4
)

func (f HeaderFlags) Has(flag HeaderFlags) bool { return f&flag == flag }

func (f HeaderFlags) String() string {
	if f == HeaderFlagEmpty {
		return "EMPTY"
	}
	s := ""
	add := func(flag HeaderFlags, name string) {
		if f.Has(flag) {
			if s != "" {
				s += "|"
			}
			s += name
		}
	}
	add(HeaderFlagDirty, "DIRTY")
	add(HeaderFlagFull, "FULL")
	add(HeaderFlagNoCRC32, "NO_CRC32")
	if s == "" {
		s = "UNKNOWN"
	}
	return s
}

// FileHeader — это фиксированный структурированный префикс размером 128 байт,
// являющийся частью 4096-байтового блока заголовка файла EVTX.
type FileHeader struct {
	FirstChunkNumber uint64
	LastChunkNumber  uint64
	NextRecordID     uint64
	HeaderSize       uint32
	MinorVersion     uint16
	MajorVersion     uint16
	HeaderBlockSize  uint16

	ChunkCount uint16
	Flags      HeaderFlags

	Checksum uint32
}

func ParseFileHeader(data []byte) (*FileHeader, error) {
	if _, err := bytesutil.Slice(data, 0, 128, "EVTX file header"); err != nil {
		return nil, err
	}

	magicBytes, _ := bytesutil.Slice(data, 0, 8, "file header magic")
	var magic [8]byte
	copy(magic[:], magicBytes)
	if string(magicBytes) != fileHeaderMagic {
		return nil, &FileMagicError{Magic: magic}
	}

	firstChunkNumber, err := bytesutil.ReadU64LE(data, 8, "file_header.first_chunk_number")
	if err != nil {
		return nil, err
	}
	lastChunkNumber, err := bytesutil.ReadU64LE(data, 16, "file_header.last_chunk_number")
	if err != nil {
		return nil, err
	}
	nextRecordID, err := bytesutil.ReadU64LE(data, 24, "file_header.next_record_id")
	if err != nil {
		return nil, err
	}
	headerSize, err := bytesutil.ReadU32LE(data, 32, "file_header.header_size")
	if err != nil {
		return nil, err
	}
	minorVersion, err := bytesutil.ReadU16LE(data, 36, "file_header.minor_version")
	if err != nil {
		return nil, err
	}
	majorVersion, err := bytesutil.ReadU16LE(data, 38, "file_header.major_version")
	if err != nil {
		return nil, err
	}
	headerBlockSize, err := bytesutil.ReadU16LE(data, 40, "file_header.header_block_size")
	if err != nil {
		return nil, err
	}
	chunkCount, err := bytesutil.ReadU16LE(data, 42, "file_header.chunk_count")
	if err != nil {
		return nil, err
	}
	rawFlags, err := bytesutil.ReadU32LE(data, 120, "file_header.flags")
	if err != nil {
		return nil, err
	}
	checksum, err := bytesutil.ReadU32LE(data, 124, "file_header.checksum")
	if err != nil {
		return nil, err
	}

	return &FileHeader{
		FirstChunkNumber: firstChunkNumber,
		LastChunkNumber:  lastChunkNumber,
		NextRecordID:     nextRecordID,
		HeaderSize:       headerSize,
		MinorVersion:     minorVersion,
		MajorVersion:     majorVersion,
		HeaderBlockSize:  headerBlockSize,
		ChunkCount:       chunkCount,
		Flags:            HeaderFlags(rawFlags),
		Checksum:         checksum,
	}, nil
}

// VerifyChecksum возвращает true, если h.Checksum совпадает с контрольной суммой CRC32-IEEE
// для headerBlock[:120]. headerBlock должен содержать те же байты, которые были
// изначально переданы в ParseFileHeader.
func (h *FileHeader) VerifyChecksum(headerBlock []byte) bool {
	if len(headerBlock) < 120 {
		return false
	}
	return crc32IEEE(headerBlock[:120]) == h.Checksum
}
