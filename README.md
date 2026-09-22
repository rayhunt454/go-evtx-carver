# GO-EVTX-CARVER

Библиотека и CLI на Go для разбора журналов событий Windows (EVTX) и для
карвинга EVTX-чанков/записей напрямую из сырых образов (дисковые дампы,
дампы памяти, page file) — когда целого корректного `.evtx`-файла нет, от команды [PT ESC IR](https://ptsecurity.com/services/incident-response/).

## Возможности

- Разбор `.evtx` в JSON / JSON Lines, потоково или буферизованно.
- Устойчивость к повреждениям: битая контрольная сумма чанка или отдельная
  сломанная запись не обрывают разбор всего файла — теряется только то, что
  действительно не читается, остальное разбирается как обычно.
- Карвинг тремя стратегиями, когда файла EVTX как такового больше нет:
  - по чанкам — с восстановлением полной структуры записи, включая имена полей;
  - по отдельным записям — восстанавливает значения полей без имён (имена
    полей — ссылки в кэше строк чанка, а чанка при таком карвинге нет);
  - по чанкам, сжатым NTFS-компрессией (LZNT1) — тот же результат, что и
    карвинг по чанкам, но для случая, когда `winevt\Logs` лежал на томе со
    включённым сжатием и сигнатура чанка на диске напрямую не встречается.
- CLI-обёртка над обоими режимами.

## Как подключить в свой проект

Пакеты `evtx` и `carve` — на верхнем уровне модуля (не под `internal/`),
поэтому импортируются извне как обычная библиотека. Реализация (декодирование
BinXML, рендеринг JSON и т.д.) лежит в `internal/*` и снаружи модуля
недоступна — `evtx` и `carve` единственное, что нужно внешнему коду.

```bash
go get github.com/rayhunt454/go-evtx-carver
```

Импорт в коде одинаков в обоих случаях:

```go
import (
    "github.com/rayhunt454/go-evtx-carver/evtx"
    "github.com/rayhunt454/go-evtx-carver/carve"
)
```
### `Parse` — самый простой вход, один вызов

```go
func Parse(path string, validateChecksums bool) ([]json.RawMessage, Stats, error)
```

Разбирает файл целиком, буферизуя весь результат в память. Возвращает JSON
каждой записи и итоговую статистику. Ошибка — только если файл не удалось
открыть; ошибки разбора отдельных записей не фатальны и учтены в
`Stats.ErrorCount`.

```go
records, stats, err := evtx.Parse("security.evtx", true)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("разобрано %d записей, ошибок: %d\n", stats.RecordCount, stats.ErrorCount)
for _, r := range records {
    fmt.Println(string(r)) // r — json.RawMessage, одна запись
}
```

### `ParseFileToJSONL` — потоковый вариант для больших файлов

```go
func ParseFileToJSONL(inputPath string, validateChecksums bool, warnOut io.Writer) (<-chan []byte, <-chan Stats, error)
```

Держит в памяти один 64 КБ чанк за раз вместо всего файла. `Parse` — это
удобная обёртка поверх неё же. Канал `lines` отдаёт JSON каждой записи (без
`\n`) по одной, в порядке следования, и закрывается по завершении; после
этого в `statsCh` приходит одно итоговое значение `Stats`. `warnOut` (может
быть `nil`) получает по строке диагностики на каждую пропущенную ошибку.

```go
lines, statsCh, err := evtx.ParseFileToJSONL("security.evtx", true, os.Stderr)
if err != nil {
    log.Fatal(err)
}
for line := range lines {
    // обработать line ([]byte, JSON одной записи) сразу, не накапливая в память
}
stats := <-statsCh
```

`validateChecksums=true` не отбрасывает чанк из-за несовпавшей контрольной
суммы целиком — такой чанк всё равно разбирается (несовпадение — лишь
предупреждение), чтобы не терять потенциально значимые записи из-за
частичного повреждения файла.

### `ParseFileToMap`/`ParseFileToMapStream` — для кастомных анализаторов логов

```go
func ParseFileToMap(path string, validateChecksums bool) ([]EventMap, Stats, error)
func ParseFileToMapStream(path string, validateChecksums bool, warnOut io.Writer) (<-chan EventMap, <-chan Stats, error)
```

Как `Parse`/`ParseFileToJSONL`, но каждая запись — не JSON-байты, а
`EventMap` (`map[string]interface{}` с типизированными значениями:
int64/uint64/bool/string/…, без промежуточной сериализации в JSON и обратно).
`ParseFileToMap` — буферизованная обёртка над `ParseFileToMapStream`, как
`Parse` над `ParseFileToJSONL`: удобнее, но не отдаёт ни одной записи, пока не
разберёт файл целиком. `ParseFileToMapStream` — потоково, по одной записи за
раз, без буферизации всего файла в память; так можно, например, остановить
чтение сразу, как только нашлась нужная запись, не дожидаясь конца файла:

```go
events, _, err := evtx.ParseFileToMapStream("security.evtx", true, nil)
for e := range events {
    if id, ok := e.GetUint64("EventID"); ok && id == 4624 {
        // нашли — можно break/return, не дочитывая файл
        break
    }
}
```

(Если выйти из цикла раньше конца канала, фоновая горутина зависает на
отправке следующей записи — вызывающему коду это не мешает, но лучше при
таком раннем выходе не забывать об утечке горутины/файлового дескриптора, см.
комментарий `ParseFileToMapStream`.)

`(EventMap).Get(key string) (interface{}, bool)` ищет поле по имени: без
точки — рекурсивно по всему дереву записи, независимо от вложенности (не
нужно знать, что `EventID` лежит внутри `System`); с точкой — точный путь от
корня события без ведущего `Event.` (например `"System.EventID"`) — на
случай, когда имя поля неоднозначно (встречается в записи не один раз).
`GetString`/`GetUint64`/`GetInt64`/`GetBool` — то же самое с приведением к
конкретному типу.

### Ручной обход на уровне структуры

Когда нужен доступ к файлу/чанку/записи напрямую, а не готовый JSON:

```go
p, err := evtx.Open("security.evtx")
// или evtx.NewParser(r evtx.ReadSeeker) — если данные не в файле на диске
if err != nil {
    log.Fatal(err)
}
defer p.Close()

chunks := p.Chunks()
for {
    chunk, err := chunks.Next() // *evtx.ChunkData, error
    if chunk == nil {
        break // err == nil здесь означает конец файла
    }
    records := chunk.Records()
    for {
        rec, err := records.Next() // *evtx.Record, error
        if rec == nil {
            break
        }
        _ = rec.EventRecordID()
        _ = rec.Timestamp()
        _ = rec.RawBinXML() // сырой, ещё не разобранный BinXML
    }
}
```

Есть и `(*Parser).Records()` — плоский обход всех записей файла без ручной
вложенной работы с чанками.

## Пакет `carve` — восстановление из сырых образов

Используется, когда нет корректного `.evtx`-файла целиком (диск
зашифрован/повреждена файловая система, но можно получить сырой образ или
дамп памяти) — сканирует байты напрямую на сигнатуры EVTX.

### `Carve` — диспетчер, стратегия задаётся значением

```go
func Carve(imagePath string, mode Mode) ([]CarvedRecord, error)

const (
    ModeChunks  Mode = "chunks"  // CarveChunks
    ModeRecords Mode = "records" // CarveRecordsByMagic
    ModeChunksLZNT1 Mode = "chunks-lznt1" // CarveChunksLZNT1
)
```

```go
recs, err := carve.Carve("disk.dd", carve.ModeChunks)
```

Удобен, когда режим выбирается динамически (например, из флага CLI); в
остальных случаях можно вызывать `CarveChunks`/`CarveRecordsByMagic` напрямую.

### `CarveChunks` — карвинг по чанкам, с полной структурой

```go
func CarveChunks(imagePath string) ([]CarvedRecord, error)
```

Ищет сигнатуры чанков и декодирует записи с полной структурой, включая имена
полей (доступен кэш строк владеющего чанка). У EVTX нет чексуммы на отдельную
запись, только на чанк целиком, поэтому каждый результат получает
`Confidence` вместо бинарного accept/reject:

| `Confidence`               | Значение                                                        |
|-----------------------------|------------------------------------------------------------------|
| `ConfidenceChunkValidated`   | верны обе чексуммы чанка (заголовка и данных записей)            |
| `ConfidenceChunkHeaderOnly`  | верна чексумма заголовка, данных — нет (часто оборванный хвост)  |
| `ConfidenceChunkUnverified`  | сигнатура/заголовок распознаны, но чексумма заголовка не сошлась |

Чанк с плохой чексуммой не отбрасывается — он всё равно разбирается, просто с
пониженным `Confidence`.

### `CarveRecordsByMagic` — карвинг по отдельным записям

```go
func CarveRecordsByMagic(imagePath string) ([]CarvedRecord, error)
```

Сканирует образ на сигнатуру записи (`0x2A 0x2A 0x00 0x00`) независимо от
владеющего чанка — то есть работает и когда чанк утерян/повреждён, а сама
запись физически цела. Имена полей восстановить нельзя (они — ссылки в
кэше строк чанка), но значения подстановок восстанавливаются позиционно, в
порядке объявления (`Values`), с типом каждого значения рядом (`ValueTypes`,
например `"UInt16"`) — подсказка для сопоставления позиции с полем вроде
EventID (почти всегда `UInt16`), но не гарантия. Такие записи получают
`Confidence == ConfidenceRecordEnvelope`. Если запись не удаётся разобрать
даже как TemplateInstance, используется резервный сырой sweep текста
(`RawStrings`).

### `CarveChunksLZNT1` — карвинг чанков, сжатых NTFS-компрессией (LZNT1)

```go
func CarveChunksLZNT1(imagePath string) ([]CarvedRecord, error)
```

Для случая, когда `winevt\Logs` лежал на NTFS-томе с включённым прозрачным
сжатием (LZNT1): в этом случае байты чанка на диске — не сами данные, а поток
сжатия, и обычный `CarveChunks` там ничего не находит (сигнатуры чанка в
сжатых байтах физически нет). Единица сжатия NTFS (Compression Unit, 16
кластеров) при стандартном кластере 4 КБ равна ровно 64 КБ — то есть ровно
размеру чанка EVTX, и границы чанков файла всегда совпадают с границами этой
единицы. Из этого следует ключевой трюк: первые байты любого LZ77-потока
всегда литеральные (окну назад ссылаться ещё не на что), а значит сигнатура
чанка `ElfChnk\x00` присутствует в сжатом потоке как есть, но со сдвигом на
2 или 3 байта назад (2-байтный заголовок LZNT1-подчанка, плюс ещё 1 байт
флагов токенов, если сам подчанк тоже помечен сжатым) — от найденной сигнатуры
достаточно отступить на эти 2-3 байта и раскодировать LZNT1 до полных 64 КБ.
Восстановленный буфер дальше проверяется и разбирается той же логикой, что и
у `CarveChunks` (чексуммы, `Confidence`, обход записей) — отдельной ветки
разбора записей для сжатого случая нет.

Это отдельная, дополняющая стратегия, а не замена `CarveChunks`: несжатые
чанки она не находит (нет заголовка LZNT1-подчанка перед сигнатурой), а
`CarveChunks` не находит сжатые — если заранее неизвестно, применялось ли
сжатие, стоит запускать оба режима на одном образе. У записей, восстановленных
этим способом, всегда заполнены `CompressedSourceOffset`/`CompressedSourceBytes`
— смещение начала потока LZNT1 в образе и сколько байт этого потока
потребовалось декодеру, чтобы получить чанк целиком (то есть степень сжатия
конкретно этого чанка); для двух других стратегий `CompressedSourceOffset`
всегда `-1`.


### `CarvedRecord`

Общая структура результата для обеих стратегий — гарантированно заполнены
только `EventRecordID` и `Timestamp` (берутся из заголовка записи), остальное
по возможности:

```go
type CarvedRecord struct {
    ImageOffset int64 // смещение заголовка записи в образе
    ChunkOffset int64 // смещение владеющего чанка, или -1 (CarveRecordsByMagic)

    EventRecordID uint64
    Timestamp     time.Time

    JSON       []byte   // полный JSON записи — заполняет только CarveChunks
    Values     []string // значения подстановок позиционно — заполняет только CarveRecordsByMagic
    ValueTypes []string // wire-тип каждого Values[i], тот же порядок
    RawStrings []string // резервный сырой текст, если Values не заполнен

    Confidence Confidence
    Note       string // пояснение, если JSON пуст или Confidence снижен
}
```

### Потоковый карвинг — `*Stream`

У каждой из трёх стратегий, и у диспетчера `Carve`, есть потоковый вариант с
суффиксом `Stream`, который отдаёт записи по мере обнаружения вместо
накопления всех сразу в одном срезе:

```go
func CarveChunksStream(imagePath string) (<-chan CarvedRecord, <-chan error, error)
func CarveRecordsByMagicStream(imagePath string) (<-chan CarvedRecord, <-chan error, error)
func CarveChunksLZNT1Stream(imagePath string) (<-chan CarvedRecord, <-chan error, error)
func CarveStream(imagePath string, mode Mode) (<-chan CarvedRecord, <-chan error, error)
```

Буферизованные `CarveChunks`/`CarveRecordsByMagic`/`CarveChunksLZNT1`/`Carve`
теперь сами реализованы как обёртка над потоковым вариантом (вычитывают канал
целиком в срез) — поведение не изменилось, отличается только то, какой ценой
по памяти это достигается.

Устройство каналов одинаковое для всех четырёх функций, по аналогии с
`evtx.ParseFileToJSONL`/`ParseFileToMapStream`:

- сканирование образа идёт в фоновой горутине; каждая найденная запись
  отправляется в канал `records` сразу, без ожидания конца скана;
- канал `records` закрывается по завершении сканирования (успешном или нет),
  после чего в `errCh` приходит ровно одно значение (`nil` или ошибка скана) и
  он тоже закрывается — читать `errCh` имеет смысл только после того, как
  `records` вычитан до конца;
- если выйти из цикла по `records`, не дочитав канал до конца, фоновая
  горутина навсегда блокируется на отправке следующей записи (обычная утечка
  горутины, не паника) — дочитывайте канал (например, в фоне, если ранний
  выход всё же нужен) либо используйте буферизованный вариант, когда ранний
  выход не требуется.

```go
recs, errCh, err := carve.CarveChunksStream("disk.dd")
if err != nil {
    log.Fatal(err)
}
for rec := range recs {
    // обрабатывать rec сразу — например, писать строку JSON в файл —
    // не дожидаясь, пока просканируется весь образ и накопятся все записи.
}
if err := <-errCh; err != nil {
    log.Printf("скан не завершился полностью: %v", err)
    // записи, полученные из recs до этой ошибки, уже обработаны и не теряются.
}
```

## CLI

```bash
go build -o evtx ./cmd/evtx
```

Разбор `.evtx`-файла или всех `.evtx`-файлов в директории — в JSON Lines:

```bash
./evtx -out ./output security.evtx
./evtx -out ./output C:\logs
```

Карвинг из сырого образа:

```bash
./evtx -mode chunks       -out result.jsonl disk.dd
./evtx -mode records      -out result.jsonl disk.dd
./evtx -mode chunks-lznt1 -out result.jsonl disk.dd  # winevt\Logs со сжатием NTFS
```

Флаги:

| Флаг                              | Значение                                                                                   |
|------------------------------------|----------------------------------------------------------------------------------------------|
| `-mode chunks\|records\|chunks-lznt1` | включает карвинг соответствующей стратегией; без флага — обычный парсинг                 |
| `-out`                             | режим парсинга — директория для `<basename>.jsonl`; режим карвинга — файл (по умолчанию stdout) |
| `-validate-checksums`              | проверять CRC32 чанков (только парсинг); несовпадение — предупреждение, не потеря данных     |
| `-quiet`                            | не печатать построчные итоги, только финальную сводку                                        |

В режиме `chunks-lznt1` каждая строка JSON дополнительно содержит
`compressed_source_offset` и `compressed_source_bytes` (в остальных режимах
эти поля отсутствуют) — смещение потока LZNT1 в образе и степень сжатия
конкретного чанка, см. `CarvedRecord.CompressedSourceOffset` выше.

`go run ./cmd/evtx -h` печатает актуальную справку.

