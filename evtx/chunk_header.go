package evtx

import (
	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

// chunkHeaderMagic — это 8-байтовая сигнатура, с которой начинается каждый фрагмент EVTX.
const chunkHeaderMagic = "ElfChnk\x00"

// ChunkSize — это фиксированный размер каждого чанка EVTX на диске.
const ChunkSize = 65536

// ChunkHeaderSize — это размер структурированного заголовка чанка; данные записи
// начинаются сразу после него.
const ChunkHeaderSize = 512

// NumStringBuckets — это количество хеш-корзин в таблице строк чанка.
const NumStringBuckets = 64

// NumTemplateBuckets — это количество хеш-корзин в таблице шаблонов чанка.
const NumTemplateBuckets = 32

// ChunkFlags — это битовые флаги, хранящиеся в заголовке чанка EVTX.
type ChunkFlags uint32

const (
	ChunkFlagEmpty   ChunkFlags = 0x0
	ChunkFlagDirty   ChunkFlags = 0x1
	ChunkFlagNoCRC32 ChunkFlags = 0x4
)

func (f ChunkFlags) Has(flag ChunkFlags) bool { return f&flag == flag }

// ChunkHeader — это структурированный заголовок фиксированного размера (512 байт),
// расположенный в начале каждого фрагмента EVTX.
type ChunkHeader struct {
	FirstEventRecordNumber    uint64
	LastEventRecordNumber     uint64
	FirstEventRecordID        uint64
	LastEventRecordID         uint64
	HeaderSize                uint32
	LastEventRecordDataOffset uint32
	// FreeSpaceOffset — это смещение (от начала чанка), указывающее на позицию
	// сразу за последним допустимым байтом данных записи. На этом значении
	// итерация по записям прекращается.
	//
	// Эти данные считываются из файла и не считаются доверенными: в поврежденном
	// чанке это значение может выходить за пределы самого чанка, поэтому
	// вызывающий код должен ограничивать его величиной ChunkSize перед
	// использованием в качестве границы среза (в данном пакете итерация
	// по записям уже выполняет такую ​​проверку).
	FreeSpaceOffset     uint32
	EventsChecksum      uint32
	HeaderChunkChecksum uint32
	Flags               ChunkFlags
	// StringsOffsets — это смещения заголовков хеш-корзин таблицы строк чанка
	StringsOffsets [NumStringBuckets]uint32
	// TemplateOffsets — это смещения заголовков хеш-корзин таблицы шаблонов
	TemplateOffsets [NumTemplateBuckets]uint32
}

func ParseChunkHeader(data []byte) (*ChunkHeader, error) {
	if _, err := bytesutil.Slice(data, 0, ChunkHeaderSize, "EVTX chunk header"); err != nil {
		return nil, err
	}

	magicBytes, _ := bytesutil.Slice(data, 0, 8, "chunk header magic")
	var magic [8]byte
	copy(magic[:], magicBytes)
	if string(magicBytes) != chunkHeaderMagic {
		return nil, &ChunkMagicError{Magic: magic}
	}

	firstEventRecordNumber, err := bytesutil.ReadU64LE(data, 8, "chunk.first_event_record_number")
	if err != nil {
		return nil, err
	}
	lastEventRecordNumber, err := bytesutil.ReadU64LE(data, 16, "chunk.last_event_record_number")
	if err != nil {
		return nil, err
	}
	firstEventRecordID, err := bytesutil.ReadU64LE(data, 24, "chunk.first_event_record_id")
	if err != nil {
		return nil, err
	}
	lastEventRecordID, err := bytesutil.ReadU64LE(data, 32, "chunk.last_event_record_id")
	if err != nil {
		return nil, err
	}
	headerSize, err := bytesutil.ReadU32LE(data, 40, "chunk.header_size")
	if err != nil {
		return nil, err
	}
	lastEventRecordDataOffset, err := bytesutil.ReadU32LE(data, 44, "chunk.last_event_record_data_offset")
	if err != nil {
		return nil, err
	}
	freeSpaceOffset, err := bytesutil.ReadU32LE(data, 48, "chunk.free_space_offset")
	if err != nil {
		return nil, err
	}
	eventsChecksum, err := bytesutil.ReadU32LE(data, 52, "chunk.events_checksum")
	if err != nil {
		return nil, err
	}
	rawFlags, err := bytesutil.ReadU32LE(data, 120, "chunk.flags")
	if err != nil {
		return nil, err
	}
	headerChunkChecksum, err := bytesutil.ReadU32LE(data, 124, "chunk.header_chunk_checksum")
	if err != nil {
		return nil, err
	}

	var stringsOffsets [NumStringBuckets]uint32
	for i := range stringsOffsets {
		v, err := bytesutil.ReadU32LE(data, 128+i*4, "chunk.strings_offsets")
		if err != nil {
			return nil, err
		}
		stringsOffsets[i] = v
	}

	var templateOffsets [NumTemplateBuckets]uint32
	for i := range templateOffsets {
		v, err := bytesutil.ReadU32LE(data, 384+i*4, "chunk.template_offsets")
		if err != nil {
			return nil, err
		}
		templateOffsets[i] = v
	}

	return &ChunkHeader{
		FirstEventRecordNumber:    firstEventRecordNumber,
		LastEventRecordNumber:     lastEventRecordNumber,
		FirstEventRecordID:        firstEventRecordID,
		LastEventRecordID:         lastEventRecordID,
		HeaderSize:                headerSize,
		LastEventRecordDataOffset: lastEventRecordDataOffset,
		FreeSpaceOffset:           freeSpaceOffset,
		EventsChecksum:            eventsChecksum,
		HeaderChunkChecksum:       headerChunkChecksum,
		Flags:                     ChunkFlags(rawFlags),
		StringsOffsets:            stringsOffsets,
		TemplateOffsets:           templateOffsets,
	}, nil
}
