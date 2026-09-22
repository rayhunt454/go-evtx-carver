package evtx

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/rayhunt454/go-evtx-carver/internal/render"

	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
)

// Stats — итог работы ParseFileToJSONL/Parse: сколько записей разобрано
// и сколько ошибок пропущено (отдельная битая запись/chunk не прерывает
// разбор всего файла).
type Stats struct {
	RecordCount uint64
	ErrorCount  uint64
}

// Parse разбирает .evtx файл целиком и возвращает JSON каждой записи вместе
// со статистикой. Это обёртка над ParseFileToJSONL, буферизующая весь
// результат в память — для больших файлов используйте ParseFileToJSONL
// напрямую (потоково, без буферизации).
//
// Ошибка возвращается, только если файл не удалось открыть; ошибки разбора
// отдельных записей не фатальны и учтены в Stats.ErrorCount.
func Parse(path string, validateChecksums bool) ([]json.RawMessage, Stats, error) {
	lines, statsCh, err := ParseFileToJSONL(path, validateChecksums, nil)
	if err != nil {
		return nil, Stats{}, fmt.Errorf("evtx: %w", err)
	}

	var records []json.RawMessage
	for line := range lines {
		records = append(records, json.RawMessage(line))
	}
	return records, <-statsCh, nil
}

// ParseFileToJSONL разбирает .evtx файл по пути inputPath и передаёт JSON
// каждой записи (без завершающего \n) в канал lines, по одной записи за
// раз, в порядке следования. Функция не пишет вывод в файл сама — это
// решение остаётся за вызывающим кодом.
//
// Канал lines закрывается после последней записи, затем в statsCh приходит
// одно итоговое значение Stats и он тоже закрывается — читать статистику
// нужно только после того, как lines полностью вычитан.
//
// warnOut, если не nil, получает по одной строке диагностики на каждую
// пропущенную ошибку; nil подавляет эти сообщения (общий счётчик всё равно
// доступен через Stats.ErrorCount).
//
// validateChecksums=true не отбрасывает чанк из-за несовпавшей контрольной
// суммы — такой чанк всё равно разбирается (это лишь предупреждение), чтобы
// не терять потенциально криминалистически значимые записи из-за частичного
// повреждения файла.
func ParseFileToJSONL(inputPath string, validateChecksums bool, warnOut io.Writer) (<-chan []byte, <-chan Stats, error) {
	if warnOut == nil {
		warnOut = io.Discard
	}

	p, err := Open(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", inputPath, err)
	}
	p.Settings.ValidateChecksums = validateChecksums

	lines := make(chan []byte)
	statsCh := make(chan Stats, 1)

	go func() {
		defer p.Close()
		defer close(lines)
		defer close(statsCh)

		stats := walkRecords(p, warnOut, func(rec *Record, root *binxml.Element) error {
			line, jerr := render.JSON(root)
			if jerr != nil {
				return fmt.Errorf("rendering JSON: %w", jerr)
			}
			lines <- line
			return nil
		})
		statsCh <- stats
	}()

	return lines, statsCh, nil
}

// walkRecords проходит по всем чанкам/записям уже открытого p, разбирает
// BinXML каждой записи и вызывает handle с результатом. Общий обход для
// ParseFileToJSONL и ParseFileToMap — отличаются они только тем, во что
// handle рендерит root (JSON-байты или generic-структуру).
//
// stats.RecordCount считает записи, для которых handle не вернул ошибку;
// ошибка на любом из трёх этапов (сам chunk, сама запись, handle) не
// прерывает обход — она попадает в stats.ErrorCount и, если warnOut не nil,
// печатается туда же строкой.
func walkRecords(p *Parser, warnOut io.Writer, handle func(rec *Record, root *binxml.Element) error) Stats {
	if warnOut == nil {
		warnOut = io.Discard
	}

	var stats Stats
	chunks := p.Chunks()
	for {
		chunk, cerr := chunks.Next()
		if cerr != nil {
			stats.ErrorCount++
			fmt.Fprintf(warnOut, "warning: %v\n", cerr)
		}
		if chunk == nil {
			if cerr != nil {
				continue // чанк недоступен целиком — пробуем следующий
			}
			break
		}

		ctx := binxml.NewChunkContext(chunk.Data, nil)
		records := chunk.Records()
		for {
			rec, rerr := records.Next()
			if rerr != nil {
				stats.ErrorCount++
				fmt.Fprintf(warnOut, "error: %v\n", rerr)
				continue
			}
			if rec == nil {
				break
			}

			root, perr := ctx.ParseRecord(rec.BinXMLOffset(), rec.BinXMLSize())
			if perr != nil {
				stats.ErrorCount++
				fmt.Fprintf(warnOut, "error: record %d: %v\n", rec.EventRecordID(), perr)
				continue
			}

			if herr := handle(rec, root); herr != nil {
				stats.ErrorCount++
				fmt.Fprintf(warnOut, "error: record %d: %v\n", rec.EventRecordID(), herr)
				continue
			}
			stats.RecordCount++
		}
	}
	return stats
}

// ParseFileToMap разбирает .evtx файл целиком и возвращает EventMap каждой
// записи вместе со статистикой. Это обёртка над ParseFileToMapStream,
// буферизующая весь результат в память — как Parse относится к
// ParseFileToJSONL. Для больших файлов, или когда нужен ранний выход (нашли
// искомую запись — прекратили чтение файла), используйте ParseFileToMapStream
// напрямую: ParseFileToMap не отдаёт ни одной записи, пока не разберёт файл
// целиком.
//
// Ошибка возвращается, только если файл не удалось открыть; ошибки разбора
// отдельных записей не фатальны и учтены в Stats.ErrorCount.
func ParseFileToMap(path string, validateChecksums bool) ([]EventMap, Stats, error) {
	eventsCh, statsCh, err := ParseFileToMapStream(path, validateChecksums, nil)
	if err != nil {
		return nil, Stats{}, err
	}

	var events []EventMap
	for e := range eventsCh {
		events = append(events, e)
	}
	return events, <-statsCh, nil
}

// ParseFileToMapStream разбирает .evtx файл по пути path и передаёт EventMap
// каждой записи в канал events, по одной записи за раз, в порядке следования
// — потоково, без буферизации всего файла в память (устройство один в один
// как у ParseFileToJSONL, только на выходе не JSON-байты, а EventMap: generic-
// структура с типизированными значениями int64/uint64/bool/string/…, удобная
// для программного анализа без похода через JSON и обратно). Подходит для
// самодельных анализаторов логов — например, чтобы найти первую запись с
// нужным EventID и сразу остановиться, не дожидаясь разбора всего файла:
//
//	events, statsCh, err := evtx.ParseFileToMapStream("security.evtx", true, nil)
//	for e := range events {
//	    if id, ok := e.GetUint64("EventID"); ok && id == 4624 {
//	        // нашли — можно return/break, не читая файл дальше
//	    }
//	}
//
// Канал events закрывается после последней записи, затем в statsCh приходит
// одно итоговое значение Stats и он тоже закрывается — читать статистику
// нужно только после того, как events полностью вычитан. Если выйти из цикла
// раньше (как в примере выше) не дочитав events до конца, фоновая горутина
// блокируется на отправке в events навсегда — тогда сначала прочитайте канал
// events до закрытия (например, через `for range events {}` в defer) или
// перезапускайте разбор через ParseFileToMap на файлах, где ранний выход не
// принципиален.
//
// warnOut, если не nil, получает по одной строке диагностики на каждую
// пропущенную ошибку; nil подавляет эти сообщения (общий счётчик всё равно
// доступен через Stats.ErrorCount).
//
// validateChecksums=true не отбрасывает чанк из-за несовпавшей контрольной
// суммы — такой чанк всё равно разбирается (это лишь предупреждение), чтобы
// не терять потенциально криминалистически значимые записи из-за частичного
// повреждения файла.
func ParseFileToMapStream(path string, validateChecksums bool, warnOut io.Writer) (<-chan EventMap, <-chan Stats, error) {
	if warnOut == nil {
		warnOut = io.Discard
	}

	p, err := Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("evtx: opening %s: %w", path, err)
	}
	p.Settings.ValidateChecksums = validateChecksums

	events := make(chan EventMap)
	statsCh := make(chan Stats, 1)

	go func() {
		defer p.Close()
		defer close(events)
		defer close(statsCh)

		stats := walkRecords(p, warnOut, func(rec *Record, root *binxml.Element) error {
			m, merr := render.Map(root)
			if merr != nil {
				return fmt.Errorf("rendering map: %w", merr)
			}
			events <- EventMap(m)
			return nil
		})
		statsCh <- stats
	}()

	return events, statsCh, nil
}
