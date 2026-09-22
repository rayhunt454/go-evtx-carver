package bytesutil

// Cursor — это легковесный, проверяемый на соответствие границам последовательный считыватель над
// срезом байтов в памяти.
type Cursor struct {
	buf []byte
	pos int
}

func NewCursor(buf []byte) *Cursor {
	return &Cursor{buf: buf}
}

func NewCursorAt(buf []byte, pos int) (*Cursor, error) {
	if pos < 0 || pos > len(buf) {
		return nil, truncated("cursor.position", pos, 0, len(buf))
	}
	return &Cursor{buf: buf, pos: pos}, nil
}

func (c *Cursor) Pos() int { return c.pos }

func (c *Cursor) Len() int { return len(c.buf) }

func (c *Cursor) Remaining() int { return len(c.buf) - c.pos }

func (c *Cursor) SetPos(pos int, what string) error {
	if pos < 0 || pos > len(c.buf) {
		return truncated(what, pos, 0, len(c.buf))
	}
	c.pos = pos
	return nil
}

func (c *Cursor) TakeBytes(n int, what string) ([]byte, error) {
	b, err := Slice(c.buf, c.pos, n, what)
	if err != nil {
		return nil, err
	}
	c.pos += n
	return b, nil
}

func (c *Cursor) Peek(n int, what string) ([]byte, error) {
	return Slice(c.buf, c.pos, n, what)
}

func (c *Cursor) PeekAt(pos, n int, what string) ([]byte, error) {
	return Slice(c.buf, pos, n, what)
}

func (c *Cursor) U8(what string) (uint8, error) {
	v, err := ReadU8(c.buf, c.pos, what)
	if err != nil {
		return 0, err
	}
	c.pos++
	return v, nil
}

func (c *Cursor) U16(what string) (uint16, error) {
	v, err := ReadU16LE(c.buf, c.pos, what)
	if err != nil {
		return 0, err
	}
	c.pos += 2
	return v, nil
}

func (c *Cursor) U32(what string) (uint32, error) {
	v, err := ReadU32LE(c.buf, c.pos, what)
	if err != nil {
		return 0, err
	}
	c.pos += 4
	return v, nil
}

func (c *Cursor) U64(what string) (uint64, error) {
	v, err := ReadU64LE(c.buf, c.pos, what)
	if err != nil {
		return 0, err
	}
	c.pos += 8
	return v, nil
}
