package binxml

import (
	"bytes"
	"fmt"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

const maxPlausibleSubstitutions = 1024

// fragmentHeaderMagic — это 4-байтовый заголовок фрагмента BinXML, который отображается в каждой записи верхнего уровня.
// Запись BinXML начинается с (MS-EVEN6 §2.2.3.1): FragmentHeader
// токена (0x0f), за которым следует major=1, minor=1, flags=0.
var fragmentHeaderMagic = []byte{tokFragmentHeader, 0x01, 0x01, 0x00}

// TemplateInstanceValues ​​— это декодированное тело токена TemplateInstance (0x0c):
type TemplateInstanceValues struct {
	TemplateID        uint32
	TemplateDefOffset uint32

	DefinitionEmbedded bool
	Values             []Value
}

// Функция ParseStandaloneTemplateInstance декодирует экземпляр шаблона BinXML, найденный в
// самом начале данных, полностью самостоятельно — без необходимости использования собственного фрагмента.

func ParseStandaloneTemplateInstance(data []byte, ansiCodec func([]byte) string) (*TemplateInstanceValues, error) {
	body, err := stripToTemplateInstanceToken(data)
	if err != nil {
		return nil, err
	}
	return parseTemplateInstanceTokenBody(body, ansiCodec)
}

// Функция stripToTemplateInstanceToken проверяет и удаляет любой префикс, предшествующий
// тегу самого токена TemplateInstance (необязательный заголовок фрагмента, а затем
// сам байт токена), возвращая оставшиеся байты, начиная с
// поля "unknown1" токена — именно то, что вызывающая функция внутри parseFragment
// оставила бы для readTemplateInstance после обработки байта токена.
func stripToTemplateInstanceToken(data []byte) ([]byte, error) {
	rest := data
	if bytes.HasPrefix(rest, fragmentHeaderMagic) {
		rest = rest[len(fragmentHeaderMagic):]
	}
	if len(rest) < 1 || rest[0] != tokTemplateInstance {
		return nil, fmt.Errorf("binxml: standalone fragment does not start with a TemplateInstance token")
	}
	return rest[1:], nil
}

// parseTemplateInstanceTokenBody анализирует данные, начиная с поля "unknown1" токена TemplateInstance (байт токена уже удален вызывающей стороной),
// пробуя обе интерпретации структуры данных.
func parseTemplateInstanceTokenBody(data []byte, ansiCodec func([]byte) string) (*TemplateInstanceValues, error) {
	c, err := bytesutil.NewCursorAt(data, 0)
	if err != nil {
		return nil, err
	}
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
	afterOffset := c.Pos()

	if tv, ok := tryParseEmbeddedDefinition(data, afterOffset, ansiCodec); ok {
		tv.TemplateID, tv.TemplateDefOffset, tv.DefinitionEmbedded = templateID, templateDefOffset, true
		return tv, nil
	}
	if tv, ok := tryParseSubstitutionArray(data, afterOffset, ansiCodec); ok {
		tv.TemplateID, tv.TemplateDefOffset, tv.DefinitionEmbedded = templateID, templateDefOffset, false
		return tv, nil
	}
	return nil, fmt.Errorf("binxml: could not decode template instance substitution values (tried both an embedded definition and a bare back-reference at offset %d)", afterOffset)
}

// tryParseEmbeddedDefinition пытается интерпретировать "полное определение шаблона
// встроено прямо здесь": templateDefHeader, за которым сразу же
// следует ровно hdr.DataSize байтов тела шаблона
func tryParseEmbeddedDefinition(data []byte, pos int, ansiCodec func([]byte) string) (*TemplateInstanceValues, bool) {
	hdr, err := readTemplateDefHeaderAt(data, uint32(pos))
	if err != nil {
		return nil, false
	}
	bodyStart := pos + TemplateDefinitionHeaderSize
	bodyEnd := bodyStart + int(hdr.DataSize)
	if hdr.DataSize < uint32(len(fragmentHeaderMagic)) || bodyEnd > len(data) {
		return nil, false
	}

	if !bytes.HasPrefix(data[bodyStart:bodyEnd], fragmentHeaderMagic) {
		return nil, false
	}
	return tryParseSubstitutionArray(data, bodyEnd, ansiCodec)
}

// tryParseSubstitutionArray пытается декодировать data[pos:] как
// массив подстановок TemplateInstance (count + дескрипторы + значения,
// точно такую ​​же форму, которую readTemplateInstance декодирует, когда читатель, использующий фрагмент данных,
// уже знает, где он начинается) и проверяет результат, требуя, чтобы он
// фактически находился в конце данных
func tryParseSubstitutionArray(data []byte, pos int, ansiCodec func([]byte) string) (*TemplateInstanceValues, bool) {
	c, err := bytesutil.NewCursorAt(data, pos)
	if err != nil {
		return nil, false
	}
	numSubs, err := c.U32("number_of_substitutions")
	if err != nil || numSubs > maxPlausibleSubstitutions {
		return nil, false
	}

	type descriptor struct {
		size  uint16
		vtype ValueType
	}
	descriptors := make([]descriptor, 0, numSubs)
	for i := uint32(0); i < numSubs; i++ {
		size, err := c.U16("substitution_size")
		if err != nil {
			return nil, false
		}
		vtByte, err := c.U8("substitution_value_type")
		if err != nil {
			return nil, false
		}
		vt, ok := validValueType(vtByte)
		if !ok {
			return nil, false
		}
		if _, err := c.U8("substitution_padding"); err != nil {
			return nil, false
		}
		descriptors = append(descriptors, descriptor{size: size, vtype: vt})
	}

	values := make([]Value, 0, len(descriptors))
	for _, d := range descriptors {
		before := c.Pos()
		v, err := readValue(c, d.vtype, true, int(d.size), ansiCodec, data)
		if err != nil {
			return nil, false
		}

		if v.Type == NullType {
			if err := c.SetPos(before+int(d.size), "null_substitution_skip"); err != nil {
				return nil, false
			}
		}
		if expected := before + int(d.size); c.Pos() != expected {
			if err := c.SetPos(expected, "resync_after_substitution"); err != nil {
				return nil, false
			}
		}
		values = append(values, v)
	}

	if !endsPlausibly(data[c.Pos():]) {
		return nil, false
	}
	return &TemplateInstanceValues{Values: values}, true
}

// endsPlausibly сообщает, выглядит ли tail — то, что осталось в данных после
// последнего декодированного значения подстановки — как конец реальной записи.
func endsPlausibly(tail []byte) bool {
	if len(tail) == 0 {
		return true
	}
	if len(tail) > 32 {
		return false
	}
	return tail[0] == tokEOF
}
