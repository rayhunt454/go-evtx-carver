package binxml

import (
	"fmt"

	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// cachedTemplate — это разобранное определение шаблона: дерево IR, которое может по-прежнему
// содержать узлы-заполнители, а также предварительно вычисленный флаг, указывающий, может ли оно
// когда-либо запускать расширение подстановки массива (чтобы при создании экземпляра можно было полностью пропустить этот
// обход для шаблонов, которые в нем никогда не нуждаются).
type cachedTemplate struct {
	root            *Element
	hasLiteralArray bool
}

// ChunkContext объединяет все необходимое для парсинга BinXML
type ChunkContext struct {
	Data      []byte
	strings   *stringCache
	templates map[uint32]*cachedTemplate
	ansi      func([]byte) string
}

// NewChunkContext создает контекст парсинга BinXML для одного фрагмента данных.
func NewChunkContext(data []byte, ansiCodec func([]byte) string) *ChunkContext {
	if ansiCodec == nil {
		ansiCodec = winenc.DecodeWindows1252
	}
	return &ChunkContext{
		Data:      data,
		strings:   newStringCache(data),
		templates: make(map[uint32]*cachedTemplate),
		ansi:      ansiCodec,
	}
}

func (ctx *ChunkContext) ParseRecord(binxmlOffset int, binxmlSize int) (*Element, error) {
	return ctx.parseFragment(binxmlOffset, binxmlSize, modeRecord, false, 0)
}

// Функция getOrParseTemplate возвращает кэшированное определение шаблона по адресу
// относительно фрагмента, анализируя (и кэшируя) его при первом использовании.
func (ctx *ChunkContext) getOrParseTemplate(offset uint32, depth int) (*cachedTemplate, error) {
	if t, ok := ctx.templates[offset]; ok {
		return t, nil
	}
	hdr, err := readTemplateDefHeaderAt(ctx.Data, offset)
	if err != nil {
		return nil, err
	}
	dataStart := int(offset) + TemplateDefinitionHeaderSize
	dataEnd := dataStart + int(hdr.DataSize)
	if dataEnd > len(ctx.Data) {
		return nil, fmt.Errorf("binxml: template data at offset %d out of bounds", offset)
	}
	root, err := ctx.parseFragment(dataStart, int(hdr.DataSize), modeTemplateDefinition, true, depth)
	if err != nil {
		return nil, err
	}
	t := &cachedTemplate{root: root, hasLiteralArray: elementTreeHasLiteralArray(root)}
	ctx.templates[offset] = t
	return t, nil
}

// instantiateTemplate создает экземпляр разобранного токена TemplateInstance (его
// определение найдено/кэшировано с помощью TemplateDefOffset) на основе его собственных
// значений подстановки, возвращая полностью разрешенное дерево элементов, готовое к
// встраиванию в парсинг.
func (ctx *ChunkContext) instantiateTemplate(ti *templateInstance, depth int) (*Element, error) {
	tpl, err := ctx.getOrParseTemplate(ti.TemplateDefOffset, depth)
	if err != nil {
		return nil, err
	}
	values, err := ctx.templateValuesFromRawValues(ti.Values, depth)
	if err != nil {
		return nil, err
	}
	root, _, err := cloneAndResolve(tpl.root, values)
	return root, err
}

// templateValuesFromRawValues ​​преобразует необработанные декодированные значения подстановки в
// templateValues: подстановка типа BinXmlType сама по себе является вложенным фрагментом BinXML,
// поэтому она анализируется здесь (в режиме записи) в готовый к вставке элемент;
// все остальные значения передаются без изменений.
func (ctx *ChunkContext) templateValuesFromRawValues(raw []Value, depth int) ([]templateValue, error) {
	out := make([]templateValue, 0, len(raw))
	for i := range raw {
		v := &raw[i]
		if v.Type == BinXmlType {
			if len(v.BinXML) == 0 {
				out = append(out, templateValue{value: Value{Type: NullType}})
				continue
			}
			elem, err := ctx.parseFragment(v.BinXMLOffset, len(v.BinXML), modeRecord, false, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, templateValue{element: elem})
			continue
		}
		out = append(out, templateValue{value: *v})
	}
	return out, nil
}
