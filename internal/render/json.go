package render

// json.go рендерит дерево binxml.Element в JSON (режим атрибутов по
// умолчанию, без "--separate-json-attributes"). Атрибуты выводятся как
// соседние ключи объекта элемента (без обёртки "#attributes"), а атрибут
// "xmlns" отбрасывается — он одинаков для всех записей и не несёт
// информации (см. attrFlags и цикл по атрибутам в elementBody).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

const hexUpper = "0123456789ABCDEF"
const hexLower = "0123456789abcdef"

// JSON рендерит root как один JSON-объект: {"<RootName>": <value>}.
func JSON(root *binxml.Element) ([]byte, error) {
	r := &jsonRenderer{}
	if err := r.renderRoot(root); err != nil {
		return nil, err
	}
	out := make([]byte, r.buf.Len())
	copy(out, r.buf.Bytes())
	return out, nil
}

type jsonRenderer struct {
	buf bytes.Buffer
}

func (r *jsonRenderer) renderRoot(root *binxml.Element) error {
	r.buf.WriteByte('{')
	r.buf.WriteByte('"')
	r.buf.WriteString(root.Name)
	r.buf.WriteString("\":")
	if err := r.writeElementValue(root, false); err != nil {
		return err
	}
	r.buf.WriteByte('}')
	return nil
}

// --- вспомогательные writer-ы ---

func (r *jsonRenderer) keyWithSuffix(name string, suffix int) {
	r.buf.WriteByte('"')
	r.buf.WriteString(name)
	if suffix > 0 {
		r.buf.WriteByte('_')
		r.buf.WriteString(strconv.Itoa(suffix))
	}
	r.buf.WriteString("\":")
}

// jsonEscapedStr пишет содержимое JSON-строки s (без окружающих кавычек),
// используя экранирование encoding/json.
func (r *jsonRenderer) jsonEscapedStr(s string) {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		r.buf.Write(b[1 : len(b)-1])
	}
}

func (r *jsonRenderer) writeU64(v uint64) { r.buf.WriteString(strconv.FormatUint(v, 10)) }
func (r *jsonRenderer) writeI64(v int64)  { r.buf.WriteString(strconv.FormatInt(v, 10)) }

func (r *jsonRenderer) writeFloat32(v float32) {
	r.buf.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
}
func (r *jsonRenderer) writeFloat64(v float64) {
	r.buf.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
}

func (r *jsonRenderer) writeHexUpper(b []byte) {
	for _, c := range b {
		r.buf.WriteByte(hexUpper[c>>4])
		r.buf.WriteByte(hexUpper[c&0x0f])
	}
}

func (r *jsonRenderer) writeHexPrefixedLower(v uint64) {
	r.buf.WriteString("0x")
	if v == 0 {
		r.buf.WriteByte('0')
		return
	}
	var tmp [16]byte
	n := 0
	for v != 0 {
		tmp[n] = hexLower[v&0xf]
		n++
		v >>= 4
	}
	for i := n - 1; i >= 0; i-- {
		r.buf.WriteByte(tmp[i])
	}
}

func (r *jsonRenderer) writeDateTime(v *binxml.Value) {
	// Формат ISO 8601 с точностью до микросекунд (всегда 6 знаков дробной части).
	r.buf.WriteString(v.Time.UTC().Format("2006-01-02T15:04:05.000000Z"))
}

// valueAsNumber пишет v как "голое" число/bool JSON и сообщает, удалось ли
// это (только для Int8..UInt64 и Bool; остальные типы, включая float,
// GUID/SID/binary/time и HexInt32/64, всегда рендерятся строкой в кавычках).
func (r *jsonRenderer) valueAsNumber(v *binxml.Value) bool {
	switch v.Type {
	case binxml.Int8Type, binxml.Int16Type, binxml.Int32Type, binxml.Int64Type:
		r.writeI64(v.I64)
		return true
	case binxml.UInt8Type, binxml.UInt16Type, binxml.UInt32Type, binxml.UInt64Type:
		r.writeU64(v.U64)
		return true
	case binxml.BoolType:
		if v.Bool {
			r.buf.WriteString("true")
		} else {
			r.buf.WriteString("false")
		}
		return true
	default:
		return false
	}
}

func (r *jsonRenderer) writeList(n int, f func(i int) error) error {
	for i := 0; i < n; i++ {
		if i > 0 {
			r.buf.WriteByte(',')
		}
		if err := f(i); err != nil {
			return err
		}
	}
	return nil
}

// writeValueText пишет текстовое представление v (без кавычек): для
// значений внутри смешанного текстового содержимого и для строкового
// случая значений, не подошедших под "голое число". Массивы рендерятся как
// элементы через запятую (без скобок), каждый — как соответствующий
// скалярный тип.
func (r *jsonRenderer) writeValueText(v *binxml.Value) error {
	switch v.Type {
	case binxml.NullType:
		return nil
	case binxml.StringType:
		r.jsonEscapedStr(v.Str.String())
		return nil
	case binxml.AnsiStringType:
		r.jsonEscapedStr(v.AStr)
		return nil
	case binxml.Int8Type, binxml.Int16Type, binxml.Int32Type, binxml.Int64Type:
		r.writeI64(v.I64)
		return nil
	case binxml.UInt8Type, binxml.UInt16Type, binxml.UInt32Type, binxml.UInt64Type:
		r.writeU64(v.U64)
		return nil
	case binxml.Real32Type:
		r.writeFloat32(v.F32)
		return nil
	case binxml.Real64Type:
		r.writeFloat64(v.F64)
		return nil
	case binxml.BoolType:
		if v.Bool {
			r.buf.WriteString("true")
		} else {
			r.buf.WriteString("false")
		}
		return nil
	case binxml.BinaryType:
		r.writeHexUpper(v.Bin)
		return nil
	case binxml.GuidType:
		r.buf.WriteString(winenc.FormatGUID(v.Guid))
		return nil
	case binxml.FileTimeType, binxml.SysTimeType:
		r.writeDateTime(v)
		return nil
	case binxml.SidType:
		r.buf.WriteString(winenc.FormatSID(v.Sid))
		return nil
	case binxml.HexInt32Type, binxml.HexInt64Type:
		r.writeHexPrefixedLower(v.U64)
		return nil
	case binxml.StringArrayType:
		return r.writeList(len(v.StrArray), func(i int) error {
			r.jsonEscapedStr(v.StrArray[i].String())
			return nil
		})
	case binxml.Int8ArrayType, binxml.Int16ArrayType, binxml.Int32ArrayType, binxml.Int64ArrayType:
		return r.writeList(len(v.I64Array), func(i int) error { r.writeI64(v.I64Array[i]); return nil })
	case binxml.UInt8ArrayType, binxml.UInt16ArrayType, binxml.UInt32ArrayType, binxml.UInt64ArrayType:
		return r.writeList(len(v.U64Array), func(i int) error { r.writeU64(v.U64Array[i]); return nil })
	case binxml.Real32ArrayType:
		return r.writeList(len(v.F32Array), func(i int) error { r.writeFloat32(v.F32Array[i]); return nil })
	case binxml.Real64ArrayType:
		return r.writeList(len(v.F64Array), func(i int) error { r.writeFloat64(v.F64Array[i]); return nil })
	case binxml.BoolArrayType:
		return r.writeList(len(v.BoolArray), func(i int) error {
			if v.BoolArray[i] {
				r.buf.WriteString("true")
			} else {
				r.buf.WriteString("false")
			}
			return nil
		})
	case binxml.GuidArrayType:
		return r.writeList(len(v.GuidArray), func(i int) error {
			r.buf.WriteString(winenc.FormatGUID(v.GuidArray[i]))
			return nil
		})
	case binxml.FileTimeArrayType, binxml.SysTimeArrayType:
		return r.writeList(len(v.TimeArray), func(i int) error {
			r.buf.WriteString(v.TimeArray[i].UTC().Format("2006-01-02T15:04:05.000000Z"))
			return nil
		})
	case binxml.SidArrayType:
		return r.writeList(len(v.SidArray), func(i int) error {
			r.buf.WriteString(winenc.FormatSID(v.SidArray[i]))
			return nil
		})
	case binxml.HexInt32ArrayType, binxml.HexInt64ArrayType:
		return r.writeList(len(v.U64Array), func(i int) error { r.writeHexPrefixedLower(v.U64Array[i]); return nil })
	default:
		return fmt.Errorf("json: unsupported value type 0x%02x in renderer", byte(v.Type))
	}
}

// --- анализ содержимого ---

func nodeIsContent(n binxml.Node) bool {
	switch n.Kind {
	case binxml.KindText, binxml.KindCData:
		return n.Text != ""
	case binxml.KindEntityRef, binxml.KindCharRef:
		return true
	case binxml.KindValue:
		return !n.Value.IsOptionalEmpty()
	case binxml.KindPlaceholder:
		// В материализованном дереве такого быть не должно; считаем контентом,
		// чтобы рендеринг явно вернул ошибку, а не тихо выдал неверный результат.
		return true
	default: // KindElement, KindPITarget, KindPIData
		return false
	}
}

func hasTextContent(nodes []binxml.Node) bool {
	for _, n := range nodes {
		if nodeIsContent(n) {
			return true
		}
	}
	return false
}

// contentLayout возвращает (hasText, hasElementChild) для детей e.
func contentLayout(e *binxml.Element) (bool, bool) {
	hasText := false
	hasElementChild := e.HasElementChild
	for _, n := range e.Children {
		if n.Kind == binxml.KindElement {
			hasElementChild = true
		} else if nodeIsContent(n) {
			hasText = true
		}
		if hasText && hasElementChild {
			break
		}
	}
	return hasText, hasElementChild
}

// isSuppressedAttr — атрибуты, которые никогда не рендерятся (сейчас
// только "xmlns"). Фильтруется здесь, а не при декодировании BinXML, чтобы
// e.Attrs по-прежнему отражал реальное содержимое записи.
func isSuppressedAttr(name string) bool { return name == "xmlns" }

// attrFlags возвращает (hasAny, hasAnyNonEmptyValue) для attrs, игнорируя
// подавленные атрибуты (isSuppressedAttr) — они не должны влиять на то,
// схлопывается ли элемент в лист/null или разворачивается в объект.
func attrFlags(attrs []binxml.Attr) (bool, bool) {
	hasAny := false
	for _, a := range attrs {
		if isSuppressedAttr(a.Name) {
			continue
		}
		hasAny = true
		if hasTextContent(a.Value) {
			return true, true
		}
	}
	return hasAny, false
}

func resolveEntity(name string) (string, bool) {
	switch name {
	case "quot":
		return "\"", true
	case "apos":
		return "'", true
	case "amp":
		return "&", true
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	default:
		return "", false
	}
}

// unresolvedPlaceholder возвращается при встрече узла Placeholder во время
// рендеринга — для корректно материализованного дерева это внутренняя
// ошибка, а не проблема входных данных.
func unresolvedPlaceholder() error {
	return fmt.Errorf("json: internal error: unresolved template placeholder in materialized tree")
}

// textContent пишет текстовое содержимое nodes без окружающих кавычек.
func (r *jsonRenderer) textContent(nodes []binxml.Node, skipElements bool) error {
	for _, n := range nodes {
		switch n.Kind {
		case binxml.KindText, binxml.KindCData:
			if n.Text != "" {
				r.jsonEscapedStr(n.Text)
			}
		case binxml.KindValue:
			if err := r.writeValueText(n.Value); err != nil {
				return err
			}
		case binxml.KindCharRef:
			ch := rune(n.CharRef)
			if utf8.ValidRune(ch) {
				r.jsonEscapedStr(string(ch))
			} else {
				r.buf.WriteString("&#")
				r.writeU64(uint64(n.CharRef))
				r.buf.WriteByte(';')
			}
		case binxml.KindEntityRef:
			if resolved, ok := resolveEntity(n.Text); ok {
				r.jsonEscapedStr(resolved)
			} else {
				r.buf.WriteByte('&')
				r.buf.WriteString(n.Text)
				r.buf.WriteByte(';')
			}
		case binxml.KindPITarget, binxml.KindPIData:
			// ничего не выводим
		case binxml.KindPlaceholder:
			return unresolvedPlaceholder()
		case binxml.KindElement:
			if skipElements {
				continue
			}
			return fmt.Errorf("json: unexpected element node in text context")
		}
	}
	return nil
}

// tryAsNumber пишет nodes как "голое" число/bool JSON, если они схлопываются
// ровно в один числовой/bool узел без прочего содержимого, и сообщает,
// удалось ли это.
func (r *jsonRenderer) tryAsNumber(nodes []binxml.Node, skipElements bool) (bool, error) {
	var single *binxml.Node
	for i := range nodes {
		n := &nodes[i]
		switch n.Kind {
		case binxml.KindElement:
			if skipElements {
				continue
			}
			return false, nil
		case binxml.KindText, binxml.KindCData:
			if skipElements {
				if n.Text != "" {
					return false, nil
				}
			} else if single != nil || n.Text != "" {
				return false, nil
			} else {
				single = n
			}
		case binxml.KindValue:
			if skipElements && n.Value.IsOptionalEmpty() {
				continue
			}
			if single != nil {
				return false, nil
			}
			single = n
		case binxml.KindCharRef, binxml.KindEntityRef:
			return false, nil
		case binxml.KindPITarget, binxml.KindPIData:
			if !skipElements && single != nil {
				return false, nil
			}
			if !skipElements {
				single = n
			}
		case binxml.KindPlaceholder:
			if skipElements {
				return false, unresolvedPlaceholder()
			}
			return false, nil
		}
	}
	if single != nil && single.Kind == binxml.KindValue {
		return r.valueAsNumber(single.Value), nil
	}
	return false, nil
}

func (r *jsonRenderer) contentAsJSONValue(nodes []binxml.Node, skipElements bool) error {
	wrote, err := r.tryAsNumber(nodes, skipElements)
	if err != nil {
		return err
	}
	if wrote {
		return nil
	}
	r.buf.WriteByte('"')
	if err := r.textContent(nodes, skipElements); err != nil {
		return err
	}
	r.buf.WriteByte('"')
	return nil
}

// --- атрибуты ---

// --- значение элемента / упрощение листьев ---

func (r *jsonRenderer) tryLeafValue(e *binxml.Element, emptyAsString bool) (bool, error) {
	if e.HasElementChild || len(e.Children) != 1 {
		return false, nil
	}
	empty := "null"
	if emptyAsString {
		empty = "\"\""
	}
	n := e.Children[0]
	switch n.Kind {
	case binxml.KindText, binxml.KindCData:
		if n.Text == "" {
			r.buf.WriteString(empty)
		} else {
			r.buf.WriteByte('"')
			r.jsonEscapedStr(n.Text)
			r.buf.WriteByte('"')
		}
		return true, nil
	case binxml.KindValue:
		if n.Value.IsOptionalEmpty() {
			r.buf.WriteString(empty)
			return true, nil
		}
		if r.valueAsNumber(n.Value) {
			return true, nil
		}
		r.buf.WriteByte('"')
		if err := r.writeValueText(n.Value); err != nil {
			return false, err
		}
		r.buf.WriteByte('"')
		return true, nil
	default:
		return false, nil
	}
}

// dataElementValue рендерит значение элемента `<Data>` (при выравнивании/
// позиционной группировке детей Data внутри EventData/UserData): пустое
// значение рендерится как `""`, а не `null`.
func (r *jsonRenderer) dataElementValue(e *binxml.Element) error {
	wrote, err := r.tryLeafValue(e, true)
	if err != nil {
		return err
	}
	if wrote {
		return nil
	}
	hasText, hasElementChild := contentLayout(e)
	if !hasText && !hasElementChild {
		r.buf.WriteString("\"\"")
		return nil
	}
	if hasElementChild {
		return r.elementBody(e, false, true, hasText)
	}
	return r.contentAsJSONValue(e.Children, false)
}

// writeElementValue рендерит ЗНАЧЕНИЕ element (ключ, если он есть, —
// забота вызывающего кода). Точка входа для корневого элемента и для
// каждого обычного дочернего элемента.
func (r *jsonRenderer) writeElementValue(e *binxml.Element, childIsContainer bool) error {
	attrs := e.Attrs
	if isEventIDElementName(e.Name) {
		attrs = nil
	}
	hasAnyAttr, hasAttrsText := attrFlags(attrs)
	if !hasAnyAttr {
		wrote, err := r.tryLeafValue(e, false)
		if err != nil {
			return err
		}
		if wrote {
			return nil
		}
	}
	hasText, hasElementChild := contentLayout(e)

	if !hasElementChild && !hasText && !hasAttrsText {
		r.buf.WriteString("null")
		return nil
	}
	if !hasElementChild && !hasAttrsText {
		return r.contentAsJSONValue(e.Children, false)
	}
	return r.elementBody(e, childIsContainer, false, hasText)
}

func isDataContainerName(name string) bool { return name == "EventData" || name == "UserData" }
func isDataElementName(name string) bool   { return name == "Data" }

// isEventIDElementName — специальный случай: у EventID изредка (в "классических"
// событиях, сконвертированных из старого Event Log API) есть атрибут
// Qualifiers. Без этого правила один и тот же по смыслу код события рендерился
// бы то как голое число ("EventID":600), то как объект с "#text" и "Qualifiers"
// — в зависимости от одной записи. Атрибуты EventID сознательно игнорируются
// на рендеринге (см. writeElementValue/elementValueNative), чтобы поле всегда
// было голым числом, независимо от наличия Qualifiers.
func isEventIDElementName(name string) bool { return name == "EventID" }

func findAttr(attrs []binxml.Attr, name string) ([]binxml.Node, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return nil, false
}

// elementBody рендерит element как JSON-объект: `{<ключи атрибутов...>,
// "#text":..., <дети...>}`, применяя правила выравнивания/группировки
// "Data" внутри EventData/UserData при inDataContainer. Атрибуты пишутся
// как соседние ключи (без обёртки "#attributes") и используют общий с
// дочерними элементами счётчик nameCounts для суффиксов "_N" при коллизии
// имён.
func (r *jsonRenderer) elementBody(e *binxml.Element, inDataContainer, omitAttributes, hasText bool) error {
	shouldFlattenNamedData := false
	if inDataContainer {
		for _, n := range e.Children {
			if n.Kind != binxml.KindElement || !isDataElementName(n.Element.Name) {
				continue
			}
			nameNodes, ok := findAttr(n.Element.Attrs, "Name")
			if !ok {
				continue
			}
			if hasTextContent(nameNodes) {
				shouldFlattenNamedData = true
				break
			}
		}
	}

	nameCounts := make(map[string]int)

	r.buf.WriteByte('{')
	wroteAny := false

	if !omitAttributes {
		for _, attr := range e.Attrs {
			if isSuppressedAttr(attr.Name) || !hasTextContent(attr.Value) {
				continue
			}
			if wroteAny {
				r.buf.WriteByte(',')
			}
			wroteAny = true
			suffix := nameCounts[attr.Name]
			nameCounts[attr.Name] = suffix + 1
			r.keyWithSuffix(attr.Name, suffix)
			wroteNum, err := r.tryAsNumber(attr.Value, false)
			if err != nil {
				return err
			}
			if wroteNum {
				continue
			}
			r.buf.WriteByte('"')
			if err := r.textContent(attr.Value, false); err != nil {
				return err
			}
			r.buf.WriteByte('"')
		}
	}

	if hasText {
		if wroteAny {
			r.buf.WriteByte(',')
		}
		wroteAny = true
		r.buf.WriteString("\"#text\":")
		if err := r.contentAsJSONValue(e.Children, true); err != nil {
			return err
		}
	}

	positionalDataCount := 0
	if inDataContainer && !shouldFlattenNamedData {
		for _, n := range e.Children {
			if n.Kind == binxml.KindElement && isDataElementName(n.Element.Name) {
				positionalDataCount++
			}
		}
	}
	positionalDataEmitted := false

	for _, n := range e.Children {
		if n.Kind != binxml.KindElement {
			continue
		}
		child := n.Element

		if inDataContainer && isDataElementName(child.Name) {
			if shouldFlattenNamedData {
				nameNodes, ok := findAttr(child.Attrs, "Name")
				if !ok || !hasTextContent(nameNodes) {
					continue
				}
				if wroteAny {
					r.buf.WriteByte(',')
				}
				wroteAny = true
				r.buf.WriteByte('"')
				if err := r.textContent(nameNodes, false); err != nil {
					return err
				}
				r.buf.WriteString("\":")
				if err := r.dataElementValue(child); err != nil {
					return err
				}
			} else if !positionalDataEmitted && positionalDataCount > 0 {
				if wroteAny {
					r.buf.WriteByte(',')
				}
				wroteAny = true
				positionalDataEmitted = true
				r.buf.WriteString("\"Data\":{\"#text\":")
				if positionalDataCount == 1 {
					if err := r.dataElementValue(child); err != nil {
						return err
					}
				} else {
					r.buf.WriteByte('[')
					first := true
					for _, n2 := range e.Children {
						if n2.Kind != binxml.KindElement || !isDataElementName(n2.Element.Name) {
							continue
						}
						if !first {
							r.buf.WriteByte(',')
						}
						first = false
						if err := r.dataElementValue(n2.Element); err != nil {
							return err
						}
					}
					r.buf.WriteByte(']')
				}
				r.buf.WriteByte('}')
			}
			continue
		}

		cname := child.Name
		suffix := nameCounts[cname]
		nameCounts[cname] = suffix + 1

		if wroteAny {
			r.buf.WriteByte(',')
		}
		wroteAny = true

		childIsContainer := isDataContainerName(cname)
		r.keyWithSuffix(cname, suffix)
		if err := r.writeElementValue(child, childIsContainer); err != nil {
			return err
		}
	}

	r.buf.WriteByte('}')
	return nil
}
