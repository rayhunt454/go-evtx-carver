package binxml

import (
	"fmt"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
)

// buildMode выбирает способ интерпретации потока токенов
type buildMode int

const (
	modeRecord buildMode = iota
	modeTemplateDefinition
)

// Жесткие ограничения, лимитирующие рекурсию/вложенность при обработке искусственно созданных или поврежденных входных данных.
const (
	maxBinxmlNesting = 64
	maxElementDepth  = 512
)

// elementBuilder постепенно собирает атрибуты одного открытого элемента, пока
// анализируется его span OpenStartElement...CloseStartElement.
type elementBuilder struct {
	name         string
	attrs        []Attr
	curAttrName  *string
	curAttrValue []Node
}

func newElementBuilder(name string) *elementBuilder {
	return &elementBuilder{name: name}
}

func (b *elementBuilder) startAttribute(name string) {
	b.finishAttrIfAny()
	n := name
	b.curAttrName = &n
}

func (b *elementBuilder) pushAttrValue(n Node) {
	if b.curAttrName != nil {
		b.curAttrValue = append(b.curAttrValue, n)
	}
}

func (b *elementBuilder) finishAttrIfAny() {
	if b.curAttrName != nil {
		if len(b.curAttrValue) > 0 {
			b.attrs = append(b.attrs, Attr{Name: *b.curAttrName, Value: b.curAttrValue})
		}
		b.curAttrName = nil
		b.curAttrValue = nil
	}
}

func (b *elementBuilder) finish() *Element {
	b.finishAttrIfAny()
	return &Element{Name: b.name, Attrs: b.attrs}
}

// treeBuilder — это потребитель токенов с сохранением состояния для одного потока байтов BinXML
// (полезная нагрузка записи, тело определения шаблона или вложенный фрагмент BinXML
type treeBuilder struct {
	ctx      *ChunkContext
	mode     buildMode
	hasDepID bool
	depth    int

	stack   []*Element // open elements, outermost first
	current *elementBuilder
	root    *Element
}

func (b *treeBuilder) processOpenStartElement(nameOffset uint32) error {
	if b.current != nil {
		return fmt.Errorf("binxml: open start element - bad parser state (already inside an open element)")
	}
	name, err := b.ctx.strings.resolve(nameOffset)
	if err != nil {
		return err
	}
	b.current = newElementBuilder(name)
	return nil
}

func (b *treeBuilder) processAttribute(nameOffset uint32) error {
	name, err := b.ctx.strings.resolve(nameOffset)
	if err != nil {
		return err
	}
	if b.current == nil {
		return fmt.Errorf("binxml: attribute - bad parser state (no open element)")
	}
	b.current.startAttribute(name)
	return nil
}

func (b *treeBuilder) processEntityRef(nameOffset uint32) error {
	name, err := b.ctx.strings.resolve(nameOffset)
	if err != nil {
		return err
	}
	return b.pushNode(Node{Kind: KindEntityRef, Text: name})
}

func (b *treeBuilder) processPITarget(nameOffset uint32) error {
	name, err := b.ctx.strings.resolve(nameOffset)
	if err != nil {
		return err
	}
	return b.pushNode(Node{Kind: KindPITarget, Text: name})
}

func (b *treeBuilder) processPIData(text string) error {
	return b.pushNode(Node{Kind: KindPIData, Text: text})
}

func (b *treeBuilder) processCloseStartElement() error {
	if len(b.stack) >= maxElementDepth {
		return fmt.Errorf("binxml: elements nested too deeply (> %d)", maxElementDepth)
	}
	if b.current == nil {
		return fmt.Errorf("binxml: close start element - bad parser state (no open element)")
	}
	elem := b.current.finish()
	b.current = nil
	b.stack = append(b.stack, elem)
	return nil
}

func (b *treeBuilder) processCloseEmptyElement() error {
	if b.current == nil {
		return fmt.Errorf("binxml: close empty element - bad parser state (no open element)")
	}
	elem := b.current.finish()
	b.current = nil
	return b.attachElement(elem)
}

func (b *treeBuilder) processCloseElement() error {
	if len(b.stack) == 0 {
		return fmt.Errorf("binxml: close element - bad parser state (empty stack)")
	}
	elem := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	return b.attachElement(elem)
}

func (b *treeBuilder) processSubstitution(idx uint16, vt ValueType, optional bool) error {
	if b.mode != modeTemplateDefinition {
		return fmt.Errorf("binxml: substitution descriptor outside template definition")
	}
	return b.pushNode(Node{Kind: KindPlaceholder, Placeholder: Placeholder{
		Index: int(idx), Type: vt, Optional: optional,
	}})
}

func (b *treeBuilder) processValue(v *Value) error {
	switch v.Type {
	case BinXmlType:
		if b.current != nil {
			return fmt.Errorf("binxml: nested BinXML inside attribute value")
		}
		if len(v.BinXML) == 0 {
			return nil
		}
		elem, err := b.ctx.parseFragment(v.BinXMLOffset, len(v.BinXML), modeRecord, false, b.depth+1)
		if err != nil {
			return err
		}
		return b.attachElement(elem)
	case EvtXmlType:
		return fmt.Errorf("unimplemented: EvtXml value")
	default:
		node, err := valueToNode(v)
		if err != nil {
			return err
		}
		return b.pushNode(node)
	}
}

func (b *treeBuilder) processTemplateInstance(ti *templateInstance) error {
	if b.mode != modeRecord {
		return fmt.Errorf("binxml: template instance inside template definition")
	}
	if b.current != nil {
		return fmt.Errorf("binxml: template instance inside attribute value")
	}
	elem, err := b.ctx.instantiateTemplate(ti, b.depth)
	if err != nil {
		return err
	}
	return b.attachElement(elem)
}

// attachElement прикрепляет элемент к самому внутреннему открытому родительскому элементу
func (b *treeBuilder) attachElement(elem *Element) error {
	if len(b.stack) > 0 {
		parent := b.stack[len(b.stack)-1]
		parent.Children = append(parent.Children, Node{Kind: KindElement, Element: elem})
		parent.HasElementChild = true
		return nil
	}
	if b.root == nil {
		b.root = elem
		return nil
	}
	if b.mode == modeRecord {
		return nil // corrupted/unusual records: ignore extra top-level roots
	}
	return fmt.Errorf("binxml: multiple root elements in template definition")
}

// Функция pushNode направляет узел контента либо в значение атрибута, который в данный момент создается,
// либо в дочерние элементы самого внутреннего открытого элемента.
func (b *treeBuilder) pushNode(n Node) error {
	if b.current != nil {
		if n.Kind == KindElement {
			return fmt.Errorf("binxml: element inside attribute value")
		}
		b.current.pushAttrValue(n)
		return nil
	}
	if len(b.stack) == 0 {
		return fmt.Errorf("binxml: value outside of element")
	}
	parent := b.stack[len(b.stack)-1]
	if n.Kind == KindElement {
		parent.HasElementChild = true
	}
	parent.Children = append(parent.Children, n)
	return nil
}

func (b *treeBuilder) finish() (*Element, error) {
	if b.current != nil {
		return nil, fmt.Errorf("binxml: unfinished element start")
	}
	if len(b.stack) != 0 {
		return nil, fmt.Errorf("binxml: unbalanced element stack")
	}
	if b.root != nil {
		return b.root, nil
	}
	switch b.mode {
	case modeRecord:

		return &Element{Name: "Event"}, nil
	default:
		return nil, fmt.Errorf("binxml: template definition is missing its root element")
	}
}

// Функция readOpenStartElementHeader считывает полезную нагрузку токена OpenStartElement
// (сам байт токена уже использован): необязательный 2-байтовый "идентификатор зависимости"
// (присутствует только в потоках определения шаблона), 4-байтовый
// размер элемента, передаваемого по сети (не используется — этот парсер не пропускает его),
// ссылка на имя элемента и — если есть атрибуты — 4-байтовый размер списка атрибутов
// (также не используется).
func readOpenStartElementHeader(c *bytesutil.Cursor, hasAttrs, hasDepID bool) (uint32, error) {
	if hasDepID {
		if _, err := c.U16("open_start_element_dependency_identifier"); err != nil {
			return 0, err
		}
	}
	if _, err := c.U32("open_start_element_data_size"); err != nil {
		return 0, err
	}
	nameOffset, err := readNameOffset(c)
	if err != nil {
		return 0, err
	}
	if hasAttrs {
		if _, err := c.U32("open_start_element_attribute_list_data_size"); err != nil {
			return 0, err
		}
	}
	return nameOffset, nil
}

func readSubstitutionDescriptor(c *bytesutil.Cursor) (uint16, ValueType, error) {
	idx, err := c.U16("substitution_index")
	if err != nil {
		return 0, 0, err
	}
	vtByte, err := c.U8("substitution_value_type")
	if err != nil {
		return 0, 0, err
	}
	vt, ok := validValueType(vtByte)
	if !ok {
		return 0, 0, fmt.Errorf("offset %d: invalid substitution value type 0x%02x", c.Pos()-1, vtByte)
	}
	return idx, vt, nil
}

// Функция parseFragment анализирует один поток токенов BinXML, начиная с абсолютного
// относительного смещения начала фрагмента, длиной ровно size байт, в заданном режиме.

// Это цикл обработки токенов, используемый каждой точкой входа:
// основной полезной нагрузкой записи, телом определения шаблона и любым вложенным фрагментом BinXML
// (значением BinXmlType или встроенной заменой экземпляра шаблона).

// - режим == modeTemplateDefinition: токены SubstitutionDescriptor становятся
// узлами-заполнителями; токены TemplateInstance отклоняются; наличие нескольких корневых
// элементов является жесткой ошибкой.
// - режим == modeRecord: токены TemplateInstance создаются
// немедленно и вставляются; токены SubstitutionDescriptor
// отклоняются; допускаются дополнительные корневые элементы верхнего уровня (мягкий отказ).
func (ctx *ChunkContext) parseFragment(start, size int, mode buildMode, hasDepID bool, depth int) (*Element, error) {
	if depth > maxBinxmlNesting {
		return nil, fmt.Errorf("binxml: fragments nested too deeply (> %d)", maxBinxmlNesting)
	}
	c, err := bytesutil.NewCursorAt(ctx.Data, start)
	if err != nil {
		return nil, err
	}

	b := &treeBuilder{ctx: ctx, mode: mode, hasDepID: hasDepID, depth: depth}

	dataRead := 0
	eof := false

	for !eof && dataRead < size {
		tokStart := c.Pos()
		tok, err := c.U8("binxml_token")
		if err != nil {
			return nil, err
		}

		switch tok {
		case tokEOF:
			eof = true

		case tokTemplateInstance:
			ti, err := readTemplateInstance(c, ctx.Data, ctx.ansi)
			if err != nil {
				return nil, err
			}
			if err := b.processTemplateInstance(ti); err != nil {
				return nil, err
			}

		case tokOpenStartElement, tokOpenStartElementHasAttr:
			nameOffset, err := readOpenStartElementHeader(c, tok == tokOpenStartElementHasAttr, hasDepID)
			if err != nil {
				return nil, err
			}
			if err := b.processOpenStartElement(nameOffset); err != nil {
				return nil, err
			}

		case tokCloseStartElement:
			if err := b.processCloseStartElement(); err != nil {
				return nil, err
			}

		case tokCloseEmptyElement:
			if err := b.processCloseEmptyElement(); err != nil {
				return nil, err
			}

		case tokCloseElement:
			if err := b.processCloseElement(); err != nil {
				return nil, err
			}

		case tokValue, tokValueVariant:
			vtByte, err := c.U8("value_type")
			if err != nil {
				return nil, err
			}
			vt, ok := validValueType(vtByte)
			if !ok {
				return nil, fmt.Errorf("offset %d: invalid value variant 0x%02x", c.Pos()-1, vtByte)
			}
			v, err := readValue(c, vt, false, 0, ctx.ansi, ctx.Data)
			if err != nil {
				return nil, err
			}
			if err := b.processValue(&v); err != nil {
				return nil, err
			}

		case tokAttribute, tokAttributeVariant:
			nameOffset, err := readNameOffset(c)
			if err != nil {
				return nil, err
			}
			if err := b.processAttribute(nameOffset); err != nil {
				return nil, err
			}

		case tokEntityRef, tokEntityRefVariant:
			nameOffset, err := readNameOffset(c)
			if err != nil {
				return nil, err
			}
			if err := b.processEntityRef(nameOffset); err != nil {
				return nil, err
			}

		case tokPITarget:
			nameOffset, err := readNameOffset(c)
			if err != nil {
				return nil, err
			}
			if err := b.processPITarget(nameOffset); err != nil {
				return nil, err
			}

		case tokPIData:
			text, err := readLenPrefixedUtf16(c, false)
			if err != nil {
				return nil, err
			}
			if err := b.processPIData(text.String()); err != nil {
				return nil, err
			}

		case tokSubstitution:
			idx, vt, err := readSubstitutionDescriptor(c)
			if err != nil {
				return nil, err
			}
			if err := b.processSubstitution(idx, vt, false); err != nil {
				return nil, err
			}

		case tokSubstitutionOptional:
			idx, vt, err := readSubstitutionDescriptor(c)
			if err != nil {
				return nil, err
			}
			if err := b.processSubstitution(idx, vt, true); err != nil {
				return nil, err
			}

		case tokFragmentHeader:
			if _, err := c.TakeBytes(3, "fragment_header"); err != nil {
				return nil, err
			}

		case tokCDataSection, tokCDataSectionVariant:
			return nil, fmt.Errorf("offset %d: unimplemented token CDataSection", c.Pos())

		case tokCharRef, tokCharRefVariant:
			return nil, fmt.Errorf("offset %d: unimplemented token CharReference", c.Pos())

		default:
			return nil, fmt.Errorf("offset %d: invalid BinXML token 0x%02x", tokStart, tok)
		}

		dataRead += c.Pos() - tokStart
	}

	return b.finish()
}
