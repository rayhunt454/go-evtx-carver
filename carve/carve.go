package carve

import "fmt"

// Mode выбирает стратегию carving'а, которую запускает Carve.
type Mode string

const (
	// ModeChunks запускает CarveChunks: поиск сигнатур chunk'ов, декодирование
	// полной структуры записей (включая имена полей).
	ModeChunks Mode = "chunks"
	// ModeRecords запускает CarveRecordsByMagic: поиск сигнатур отдельных
	// записей, восстановление значений подстановок без имён полей.
	ModeRecords Mode = "records"
	// ModeChunksLZNT1 запускает CarveChunksLZNT1: поиск chunk'ов, сжатых
	// NTFS-компрессией (LZNT1).
	ModeChunksLZNT1 Mode = "chunks-lznt1"
)

// Carve — точка входа пакета: карвит образ по пути imagePath выбранной
// стратегией mode и возвращает все восстановленные записи.
//
// Буферизует все найденные записи в памяти — для потоковой обработки без
// буферизации используйте CarveStream (см. её комментарий и комментарий
// CarveChunksStream про причину: на большом образе с достаточным числом
// совпадений сигнатуры буферизованный вариант может исчерпать память).
func Carve(imagePath string, mode Mode) ([]CarvedRecord, error) {
	records, errCh, err := CarveStream(imagePath, mode)
	if err != nil {
		return nil, err
	}
	return drainStream(records, errCh)
}

// CarveStream — потоковый вариант Carve: диспетчер, выбирающий одну из
// CarveChunksStream/CarveRecordsByMagicStream/CarveChunksLZNT1Stream по
// mode.
func CarveStream(imagePath string, mode Mode) (<-chan CarvedRecord, <-chan error, error) {
	switch mode {
	case ModeChunks:
		return CarveChunksStream(imagePath)
	case ModeRecords:
		return CarveRecordsByMagicStream(imagePath)
	case ModeChunksLZNT1:
		return CarveChunksLZNT1Stream(imagePath)
	default:
		return nil, nil, fmt.Errorf("carve: unknown mode %q (want %q, %q or %q)", mode, ModeChunks, ModeRecords, ModeChunksLZNT1)
	}
}
