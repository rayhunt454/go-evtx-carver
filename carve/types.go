package carve

import "time"

// Confidence — степень доверия к содержимому CarvedRecord: у EVTX нет
// чексуммы на отдельную запись, только на весь chunk целиком.
type Confidence int

const (
	// ConfidenceChunkValidated: у chunk'а верны и чексумма заголовка, и
	// чексумма данных записей.
	ConfidenceChunkValidated Confidence = iota
	// ConfidenceChunkHeaderOnly: чексумма заголовка chunk'а верна, но
	// чексумма данных записей — нет (обычно оборванный хвост chunk'а).
	ConfidenceChunkHeaderOnly
	// ConfidenceChunkUnverified: сигнатура и заголовок chunk'а распознаны,
	// но чексумма заголовка не сошлась — самый низкий уровень доверия среди
	// chunk-based результатов.
	ConfidenceChunkUnverified
	// ConfidenceRecordEnvelope: запись найдена сканированием по магическому
	// числу записи, без владеющего chunk'а.
	ConfidenceRecordEnvelope
)

func (c Confidence) String() string {
	switch c {
	case ConfidenceChunkValidated:
		return "chunk-validated"
	case ConfidenceChunkHeaderOnly:
		return "chunk-header-only"
	case ConfidenceChunkUnverified:
		return "chunk-unverified"
	case ConfidenceRecordEnvelope:
		return "record-envelope"
	default:
		return "unknown"
	}
}

// CarvedRecord — одна запись, восстановленная любой из стратегий carving'а.
// Только EventRecordID и Timestamp гарантированно заполнены для каждого
// результата (берутся напрямую из 24-байтового заголовка записи); остальное
// заполняется по возможности — см. JSON, Values, RawStrings и Note.
type CarvedRecord struct {
	// ImageOffset — абсолютное смещение заголовка записи в исходном образе.
	ImageOffset int64
	// ChunkOffset — абсолютное смещение заголовка владеющего chunk'а, или -1,
	// если запись найдена без него (CarveRecordsByMagic).
	ChunkOffset int64

	EventRecordID uint64
	Timestamp     time.Time

	JSON []byte

	// Values — значения подстановок, decoded CarveRecordsByMagic из тела
	// TemplateInstance записи, позиционно, в порядке объявления (без имён
	// полей). Заполняется только CarveRecordsByMagic и только когда корень
	// BinXML записи распознан как TemplateInstance; иначе nil, и фолбэком
	// служит RawStrings. CarveChunks это поле не заполняет.
	Values []string

	// ValueTypes — wire-тип (binxml.ValueType.String()) каждого значения из
	// Values, в том же порядке; ValueTypes[i] относится к Values[i]. Сам по
	// себе имени поля не даёт, но подсказывает, где искать: например, EventID
	// почти всегда UInt16, что сужает круг кандидатов среди позиций. nil
	// ровно там же, где nil и Values.
	ValueTypes []string

	// RawStrings — сырой набор текста UTF-16LE, найденного в BinXML записи.
	// Последний резервный вариант CarveRecordsByMagic, заполняется только
	// когда Values равен nil; может быть пустым. CarveChunks это поле не
	// заполняет.
	RawStrings []string

	Confidence Confidence

	// Note — пояснение простым языком, почему JSON равен nil или почему
	// снижен Confidence. Пусто, если пояснять нечего.
	Note string

	// CompressedSourceOffset — абсолютное смещение в образе, с которого
	// начат разбор потока LZNT1 (сжатие NTFS), из которого восстановлен этот
	// chunk, или -1, если запись получена не через LZNT1-декомпрессию (обычный
	// путь CarveChunks/CarveRecordsByMagic). Заполняется только
	// CarveChunksLZNT1 — см. её комментарий.
	CompressedSourceOffset int64

	// CompressedSourceBytes — сколько байт исходного (сжатого) потока,
	// начиная с CompressedSourceOffset, реально потребовалось декодеру,
	// чтобы получить полные evtx.ChunkSize байт chunk'а — то есть степень
	// сжатия конкретно этого chunk'а (CompressedSourceBytes → evtx.ChunkSize).
	// 0, если CompressedSourceOffset == -1.
	CompressedSourceBytes int
}
