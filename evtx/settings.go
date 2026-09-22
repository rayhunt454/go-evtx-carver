package evtx

type ParserSettings struct {
	// ValidateChecksums включает проверку CRC32 для контрольных сумм заголовка и
	// данных каждого чанка. По умолчанию эта функция отключена, что соответствует
	// эталонной реализации: большинство потребителей предпочитают прочитать
	// имеющиеся данные — пусть даже из слегка поврежденного файла, —
	// чем получить полный сбой операции.
	ValidateChecksums bool
}

func DefaultSettings() ParserSettings {
	return ParserSettings{ValidateChecksums: false}
}
