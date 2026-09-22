package carve

import (
	"fmt"
	"os"
)

// stream.go — общая инфраструктура для потоковых Carve*Stream: открывает
// образ, запускает сканирование в фоновой горутине

func carveStream(imagePath string, scan func(f *os.File, size int64, out chan<- CarvedRecord) error) (<-chan CarvedRecord, <-chan error, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("carve: opening %s: %w", imagePath, err)
	}
	size, err := fileSize(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("carve: statting %s: %w", imagePath, err)
	}

	records := make(chan CarvedRecord)
	errCh := make(chan error, 1)

	go func() {
		defer f.Close()
		defer close(records)
		defer close(errCh)
		errCh <- scan(f, size, records)
	}()

	return records, errCh, nil
}

func drainStream(records <-chan CarvedRecord, errCh <-chan error) ([]CarvedRecord, error) {
	var results []CarvedRecord
	for rec := range records {
		results = append(results, rec)
	}
	return results, <-errCh
}
