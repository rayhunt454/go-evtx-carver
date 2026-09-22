package evtx

import "hash/crc32"

func crc32IEEE(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
