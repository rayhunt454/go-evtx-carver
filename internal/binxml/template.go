package binxml

import (
	"fmt"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

// TemplateDefinitionHeaderSize — это фиксированный размер заголовка определения шаблона:
// next_offset(u32) + guid(16 байт) + data_size(u32).
const TemplateDefinitionHeaderSize = 24

// templateDefHeader — это фиксированный заголовок определения шаблона, хранящийся в
// начале каждого определения шаблона.
type templateDefHeader struct {
	NextOffset uint32
	GUID       [16]byte
	DataSize   uint32
}

// Функция readTemplateDefHeaderCursor считывает шаблонный заголовок из текущей позиции курсора
func readTemplateDefHeaderCursor(c *bytesutil.Cursor) (*templateDefHeader, error) {
	next, err := c.U32("next_template_offset")
	if err != nil {
		return nil, err
	}
	guidBytes, err := c.TakeBytes(16, "template_guid")
	if err != nil {
		return nil, err
	}
	var guid [16]byte
	copy(guid[:], guidBytes)
	dataSize, err := c.U32("template_data_size")
	if err != nil {
		return nil, err
	}
	return &templateDefHeader{NextOffset: next, GUID: guid, DataSize: dataSize}, nil
}

// Функция readTemplateDefHeaderAt считывает templateDefHeader с абсолютным смещением относительно блока данных.
func readTemplateDefHeaderAt(data []byte, offset uint32) (*templateDefHeader, error) {
	c, err := bytesutil.NewCursorAt(data, int(offset))
	if err != nil {
		return nil, err
	}
	return readTemplateDefHeaderCursor(c)
}

// templateInstance — это один разобранный токен TemplateInstance (0x0c): который
// определяет шаблон, на который он ссылается, плюс его полностью декодированные значения подстановки
type templateInstance struct {
	TemplateID        uint32
	TemplateDefOffset uint32
	Values            []Value
}

// Функция readTemplateInstance считывает тело токена TemplateInstance (байт токена
func readTemplateInstance(c *bytesutil.Cursor, data []byte, ansiCodec func([]byte) string) (*templateInstance, error) {
	if _, err := c.U8("template_instance_unknown"); err != nil {
		return nil, err
	}
	templateID, err := c.U32("template_id")
	if err != nil {
		return nil, err
	}
	templateDefOffset, err := c.U32("template_def_offset")
	if err != nil {
		return nil, err
	}

	// An inline (not-yet-cached) template definition is embedded right here;
	// skip over its header + body so the substitution array follows.
	if uint32(c.Pos()) == templateDefOffset {
		hdr, err := readTemplateDefHeaderCursor(c)
		if err != nil {
			return nil, err
		}
		if err := c.SetPos(c.Pos()+int(hdr.DataSize), "skip_cached_template"); err != nil {
			return nil, err
		}
	}

	numSubs, err := c.U32("number_of_substitutions")
	if err != nil {
		return nil, err
	}

	type descriptor struct {
		size  uint16
		vtype ValueType
	}
	descriptors := make([]descriptor, 0, numSubs)
	for i := uint32(0); i < numSubs; i++ {
		size, err := c.U16("substitution_size")
		if err != nil {
			return nil, err
		}
		vtByte, err := c.U8("substitution_value_type")
		if err != nil {
			return nil, err
		}
		vt, ok := validValueType(vtByte)
		if !ok {
			return nil, fmt.Errorf("offset %d: invalid substitution value type 0x%02x", c.Pos()-1, vtByte)
		}
		if _, err := c.U8("substitution_padding"); err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor{size: size, vtype: vt})
	}

	values := make([]Value, 0, len(descriptors))
	for _, d := range descriptors {
		before := c.Pos()
		v, err := readValue(c, d.vtype, true, int(d.size), ansiCodec, data)
		if err != nil {
			return nil, err
		}

		if v.Type == NullType {
			if err := c.SetPos(before+int(d.size), "null_substitution_skip"); err != nil {
				return nil, err
			}
		}
		expected := before + int(d.size)
		if c.Pos() != expected {

			if err := c.SetPos(expected, "resync_after_substitution"); err != nil {
				return nil, err
			}
		}
		values = append(values, v)
	}

	return &templateInstance{TemplateID: templateID, TemplateDefOffset: templateDefOffset, Values: values}, nil
}
