package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rayhunt454/go-evtx-carver/carve"
	"github.com/rayhunt454/go-evtx-carver/evtx"
)

var letters = map[rune][]string{
	'P': {
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔═══╝ ",
		"██║     ",
		"╚═╝     ",
	},
	'T': {
		"████████╗",
		"╚══██╔══╝",
		"   ██║   ",
		"   ██║   ",
		"   ██║   ",
		"   ╚═╝   ",
	},
	'E': {
		"███████╗",
		"██╔════╝",
		"█████╗  ",
		"██╔══╝  ",
		"███████╗",
		"╚══════╝",
	},
	'S': {
		"███████╗",
		"██╔════╝",
		"███████╗",
		"╚════██║",
		"███████║",
		"╚══════╝",
	},
	'C': {
		"██████╗ ",
		"██╔════╝",
		"██║     ",
		"██║     ",
		"╚██████╗",
		" ╚═════╝",
	},
	'I': {
		"██╗",
		"██║",
		"██║",
		"██║",
		"██║",
		"╚═╝",
	},
	'R': {
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔══██╗",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	' ': {
		"  ",
		"  ",
		"  ",
		"  ",
		"  ",
		"  ",
	},
}

func renderText(text string) string {
	rows := make([]strings.Builder, 6)
	for _, ch := range text {
		glyph, ok := letters[ch]
		if !ok {
			glyph = letters[' ']
		}
		for i := 0; i < 6; i++ {
			rows[i].WriteString(glyph[i])
		}
	}
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(r.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

func main() {
	fmt.Println(renderText("PT ESC IR"))
	fmt.Println("        ·  ·  ·  ·  ·   ●   ·  ·  ·  ·  ·")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage:
  %[1]s [flags] <file.evtx | directory>
        Parse one .evtx file, or every .evtx file found under a directory
        (recursively), to JSON Lines. Each input file's output is written as
        <out>/<original-basename>.jsonl.

  %[1]s -mode <chunks|records|chunks-lznt1> [flags] <image.dd>
        Carve EVTX chunks or individual records directly out of a raw disk
        or volume image. "chunks-lznt1" is a separate
        strategy for chunks stored NTFS-compressed (LZNT1, e.g. a compressed
        winevt\Logs) — it finds only compressed chunks, "chunks" only
        uncompressed ones; run both if you don't know which applies.

flags:
`, os.Args[0])
		flag.PrintDefaults()
	}
	mode := flag.String("mode", "", `carving strategy: "chunks", "records" or "chunks-lznt1". Supplying this flag at all switches to carving mode; omit it entirely to parse well-formed .evtx file(s) instead.`)
	out := flag.String("out", ".\\", `output path. Parsing mode: a directory each input file's <basename>.jsonl is written into (created if missing; defaults to the current directory). Carving mode: a file to write JSON Lines to (defaults to stdout).`)
	validateChecksums := flag.Bool("validate-checksums", false, "validate chunk CRC32 checksums; a chunk that fails is still parsed (checksum mismatch alone never drops data), just reported as a warning (parsing mode only)")
	quiet := flag.Bool("quiet", false, "suppress the per-file/per-record summary line(s); only the final totals line is printed")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Arg(0)

	var err error
	if *mode != "" {
		err = runCarve(*mode, path, *out)
	} else {
		err = runParse(path, *out, *validateChecksums, *quiet)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// --- режим парсинга ---

// runParse находит по inputPath один или несколько .evtx-файлов (сам файл,
// либо все .evtx-файлы в директории при обходе) и парсит каждый в свой
// JSON Lines файл в outDir с именем <basename>.jsonl.
func runParse(inputPath, outDir string, validateChecksums, quiet bool) error {
	if outDir == "" {
		outDir = "."
	}
	files, err := collectEvtxFiles(inputPath)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .evtx files found under %s", inputPath)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	var totalRecords, totalErrors uint64
	var failed int
	for _, f := range files {
		outPath := filepath.Join(outDir, jsonlName(f))
		stats, ferr := parseFileToJSONLFile(f, outPath, validateChecksums)
		if ferr != nil {
			failed++
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", f, ferr)
			continue
		}
		totalRecords += stats.RecordCount
		totalErrors += stats.ErrorCount
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s -> %s: %d records (%d errors)\n", f, outPath, stats.RecordCount, stats.ErrorCount)
		}
	}
	fmt.Fprintf(os.Stderr, "total: %d/%d file(s) parsed, %d records, %d errors\n", len(files)-failed, len(files), totalRecords, totalErrors)
	if failed == len(files) {
		return fmt.Errorf("all %d file(s) failed to parse", failed)
	}
	return nil
}

// jsonlName вычисляет имя выходного файла для path: его basename с
// расширением, заменённым на .jsonl (foo.evtx -> foo.jsonl).
func jsonlName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".jsonl"
}

// collectEvtxFiles возвращает список .evtx-файлов по path
func collectEvtxFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".evtx") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", path, err)
	}
	sort.Strings(files)
	return files, nil
}

// parseFileToJSONLFile парсит path через evtx.ParseFileToJSONL и пишет
// результат в outPath. Сам ParseFileToJSONL не делает файлового I/O —
// отдаёт канал строк JSON — поэтому создание и запись outPath делается
// здесь.
func parseFileToJSONLFile(path, outPath string, validateChecksums bool) (evtx.Stats, error) {
	lines, statsCh, err := evtx.ParseFileToJSONL(path, validateChecksums, os.Stderr)
	if err != nil {
		return evtx.Stats{}, err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return evtx.Stats{}, fmt.Errorf("creating %s: %w", outPath, err)
	}
	out := bufio.NewWriterSize(f, 1<<20)
	for line := range lines {
		out.Write(line)
		out.WriteByte('\n')
	}
	if err := out.Flush(); err != nil {
		f.Close()
		return evtx.Stats{}, err
	}
	if err := f.Close(); err != nil {
		return evtx.Stats{}, err
	}

	return <-statsCh, nil
}

// --- режим карвинга ---

// runCarve пишет результат карвинга потоково — по мере того, как
// carve.CarveStream находит очередную запись, а не после того, как
// просканирован весь образ целиком. На большом образе с достаточным числом
// совпадений сигнатуры буферизация всех записей разом (как делал бы
// carve.Carve) может занять непропорционально много памяти и исчерпать её —
// см. комментарий carve.CarveChunksStream.
func runCarve(mode, imagePath, outPath string) error {
	recs, errCh, err := carve.CarveStream(imagePath, carve.Mode(mode))
	if err != nil {
		return err
	}

	w := os.Stdout
	if outPath != "" {
		f, ferr := os.Create(outPath)
		if ferr != nil {
			return fmt.Errorf("creating %s: %w", outPath, ferr)
		}
		defer f.Close()
		w = f
	}
	bw := bufio.NewWriterSize(w, 1<<20)

	var total, full, valuesOnly, degraded, envelopeOnly int
	for rec := range recs {
		line, jerr := formatCarvedRecord(rec)
		if jerr != nil {
			fmt.Fprintf(os.Stderr, "error: encoding record at image offset %d: %v\n", rec.ImageOffset, jerr)
			continue
		}
		bw.Write(line)
		bw.WriteByte('\n')
		total++

		switch {
		case rec.JSON != nil:
			full++
		case rec.Values != nil:
			valuesOnly++
		case len(rec.RawStrings) > 0:
			degraded++
		default:
			envelopeOnly++
		}
	}

	scanErr := <-errCh
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "total: %d records carved (%d full structure, %d values-only, %d raw-text fallback, %d envelope only)\n",
		total, full, valuesOnly, degraded, envelopeOnly)
	if scanErr != nil {
		return fmt.Errorf("scan did not complete: %w", scanErr)
	}
	return nil
}

// carvedRecordOut — формат JSON Lines для вывода карвинга: содержимое
// восстановленного события (если распознано) под "event", плюс метаданные
// происхождения/достоверности.
type carvedRecordOut struct {
	ImageOffset int64 `json:"image_offset"`

	ChunkOffset   int64           `json:"chunk_offset"`
	EventRecordID uint64          `json:"event_record_id"`
	Timestamp     string          `json:"timestamp"`
	Confidence    string          `json:"confidence"`
	Event         json.RawMessage `json:"event,omitempty"`

	Values []string `json:"values,omitempty"`

	ValueTypes []string `json:"value_types,omitempty"`
	RawStrings []string `json:"raw_strings,omitempty"`
	Note       string   `json:"note,omitempty"`

	CompressedSourceOffset *int64 `json:"compressed_source_offset,omitempty"`
	CompressedSourceBytes  int    `json:"compressed_source_bytes,omitempty"`
}

func formatCarvedRecord(rec carve.CarvedRecord) ([]byte, error) {
	out := carvedRecordOut{
		ImageOffset:   rec.ImageOffset,
		ChunkOffset:   rec.ChunkOffset,
		EventRecordID: rec.EventRecordID,
		Timestamp:     rec.Timestamp.Format(time.RFC3339Nano),
		Confidence:    rec.Confidence.String(),
		Values:        rec.Values,
		ValueTypes:    rec.ValueTypes,
		RawStrings:    rec.RawStrings,
		Note:          rec.Note,
	}
	if rec.CompressedSourceOffset >= 0 {
		out.CompressedSourceOffset = &rec.CompressedSourceOffset
		out.CompressedSourceBytes = rec.CompressedSourceBytes
	}
	if rec.JSON != nil {
		out.Event = json.RawMessage(rec.JSON)
	}
	return json.Marshal(out)
}
