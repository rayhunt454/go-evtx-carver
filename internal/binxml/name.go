package binxml

import "github.com/rayhunt454/go-evtx-carver/internal/bytesutil"

// Функция readNameOffset считывает "ссылку на имя" из BinXML
func readNameOffset(c *bytesutil.Cursor) (uint32, error) {
	offset, err := c.U32("name_offset")
	if err != nil {
		return 0, err
	}
	posAfter := c.Pos()
	if uint32(posAfter) == offset {

		if _, err := c.TakeBytes(6, "name_link"); err != nil {
			return 0, err
		}
		charCount, err := c.U16("string_table_name_len")
		if err != nil {
			return 0, err
		}
		total := 10 + int(charCount)*2
		if err := c.SetPos(posAfter+total, "skip_inline_name"); err != nil {
			return 0, err
		}
	}
	return offset, nil
}
