package evtx

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidChunkChecksum = errors.New("chunk data CRC32 checksum invalid")

	ErrIncompleteChunk = errors.New("reached EOF while trying to read a full chunk")
)

type FileMagicError struct {
	Magic [8]byte
}

func (e *FileMagicError) Error() string {
	return fmt.Sprintf("invalid EVTX file header magic, expected %q, found %x", fileHeaderMagic, e.Magic[:])
}

type ChunkMagicError struct {
	Magic [8]byte
}

func (e *ChunkMagicError) Error() string {
	return fmt.Sprintf("invalid EVTX chunk header magic, expected %q, found %x", chunkHeaderMagic, e.Magic[:])
}

type RecordMagicError struct {
	Magic [4]byte
}

func (e *RecordMagicError) Error() string {
	return fmt.Sprintf("invalid EVTX record header magic, expected `2a2a0000`, found `%x`", e.Magic[:])
}

func (e *RecordMagicError) IsZero() bool {
	return e.Magic == [4]byte{}
}

type InvalidDataSizeError struct {
	Length   uint32
	Expected uint32
}

func (e *InvalidDataSizeError) Error() string {
	return fmt.Sprintf("invalid EVTX record data size, should be equal to or greater than %d, found %d", e.Expected, e.Length)
}

type ChunkParseError struct {
	ChunkID uint64
	Err     error
}

func (e *ChunkParseError) Error() string {
	return fmt.Sprintf("failed to parse chunk number %d: %v", e.ChunkID, e.Err)
}

func (e *ChunkParseError) Unwrap() error { return e.Err }

type RecordParseError struct {
	RecordID uint64
	Err      error
}

func (e *RecordParseError) Error() string {
	return fmt.Sprintf("failed to parse record number %d: %v", e.RecordID, e.Err)
}

func (e *RecordParseError) Unwrap() error { return e.Err }
