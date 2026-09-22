package render

// mapval.go рендерит дерево binxml.Element в generic Go-структуру
// (map[string]interface{}/[]interface{}/string/int64/uint64/bool/nil) — те
// же правила именования полей, что и в json.go (см. его комментарий), но
// без сериализации в байты. Нужен для программного анализа записи (например,
// будущего анализатора логов), когда сырые JSON-байты неудобны.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// Map рендерит root в map[string]interface{} вида {"<RootName>": <значение>}.
func Map(root *binxml.Element) (map[string]interface{}, error) {
	v, err := elementValueNative(root, false)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{root.Name: v}, nil
}

// MapToJSON сериализует результат Map через encoding/json. В отличие от
// JSON(), порядок ключей объекта не совпадает с порядком полей записи (map
// не хранит порядок вставки, encoding/json сортирует ключи по алфавиту) —
// если порядок важен, используйте JSON().
func MapToJSON(root *binxml.Element) ([]byte, error) {
	m, err := Map(root)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func mapKey(name string, suffix int) string {
	if suffix > 0 {
		return name + "_" + strconv.Itoa(suffix)
	}
	return name
}

// valueAsNative возвращает v как "голое" число/bool (только Int8..UInt64 и
// Bool — как и valueAsNumber в json.go, остальные типы всегда строкой).
func valueAsNative(v *binxml.Value) (interface{}, bool) {
	switch v.Type {
	case binxml.Int8Type, binxml.Int16Type, binxml.Int32Type, binxml.Int64Type:
		return v.I64, true
	case binxml.UInt8Type, binxml.UInt16Type, binxml.UInt32Type, binxml.UInt64Type:
		return v.U64, true
	case binxml.BoolType:
		return v.Bool, true
	default:
		return nil, false
	}
}

// valueTextString — текстовое представление v; массивы соединяются запятой
// без разделителей-пробелов, как в writeValueText (json.go).
func valueTextString(v *binxml.Value) (string, error) {
	switch v.Type {
	case binxml.NullType:
		return "", nil
	case binxml.StringType:
		return v.Str.String(), nil
	case binxml.AnsiStringType:
		return v.AStr, nil
	case binxml.Int8Type, binxml.Int16Type, binxml.Int32Type, binxml.Int64Type:
		return strconv.FormatInt(v.I64, 10), nil
	case binxml.UInt8Type, binxml.UInt16Type, binxml.UInt32Type, binxml.UInt64Type:
		return strconv.FormatUint(v.U64, 10), nil
	case binxml.Real32Type:
		return strconv.FormatFloat(float64(v.F32), 'g', -1, 32), nil
	case binxml.Real64Type:
		return strconv.FormatFloat(v.F64, 'g', -1, 64), nil
	case binxml.BoolType:
		return strconv.FormatBool(v.Bool), nil
	case binxml.BinaryType:
		return fmt.Sprintf("%X", v.Bin), nil
	case binxml.GuidType:
		return winenc.FormatGUID(v.Guid), nil
	case binxml.FileTimeType, binxml.SysTimeType:
		return v.Time.UTC().Format("2006-01-02T15:04:05.000000Z"), nil
	case binxml.SidType:
		return winenc.FormatSID(v.Sid), nil
	case binxml.HexInt32Type, binxml.HexInt64Type:
		return "0x" + strconv.FormatUint(v.U64, 16), nil
	case binxml.StringArrayType:
		items := make([]string, len(v.StrArray))
		for i := range v.StrArray {
			items[i] = v.StrArray[i].String()
		}
		return strings.Join(items, ","), nil
	case binxml.Int8ArrayType, binxml.Int16ArrayType, binxml.Int32ArrayType, binxml.Int64ArrayType:
		items := make([]string, len(v.I64Array))
		for i := range v.I64Array {
			items[i] = strconv.FormatInt(v.I64Array[i], 10)
		}
		return strings.Join(items, ","), nil
	case binxml.UInt8ArrayType, binxml.UInt16ArrayType, binxml.UInt32ArrayType, binxml.UInt64ArrayType:
		items := make([]string, len(v.U64Array))
		for i := range v.U64Array {
			items[i] = strconv.FormatUint(v.U64Array[i], 10)
		}
		return strings.Join(items, ","), nil
	case binxml.Real32ArrayType:
		items := make([]string, len(v.F32Array))
		for i := range v.F32Array {
			items[i] = strconv.FormatFloat(float64(v.F32Array[i]), 'g', -1, 32)
		}
		return strings.Join(items, ","), nil
	case binxml.Real64ArrayType:
		items := make([]string, len(v.F64Array))
		for i := range v.F64Array {
			items[i] = strconv.FormatFloat(v.F64Array[i], 'g', -1, 64)
		}
		return strings.Join(items, ","), nil
	case binxml.BoolArrayType:
		items := make([]string, len(v.BoolArray))
		for i := range v.BoolArray {
			items[i] = strconv.FormatBool(v.BoolArray[i])
		}
		return strings.Join(items, ","), nil
	case binxml.GuidArrayType:
		items := make([]string, len(v.GuidArray))
		for i := range v.GuidArray {
			items[i] = winenc.FormatGUID(v.GuidArray[i])
		}
		return strings.Join(items, ","), nil
	case binxml.FileTimeArrayType, binxml.SysTimeArrayType:
		items := make([]string, len(v.TimeArray))
		for i := range v.TimeArray {
			items[i] = v.TimeArray[i].UTC().Format("2006-01-02T15:04:05.000000Z")
		}
		return strings.Join(items, ","), nil
	case binxml.SidArrayType:
		items := make([]string, len(v.SidArray))
		for i := range v.SidArray {
			items[i] = winenc.FormatSID(v.SidArray[i])
		}
		return strings.Join(items, ","), nil
	case binxml.HexInt32ArrayType, binxml.HexInt64ArrayType:
		items := make([]string, len(v.U64Array))
		for i := range v.U64Array {
			items[i] = "0x" + strconv.FormatUint(v.U64Array[i], 16)
		}
		return strings.Join(items, ","), nil
	default:
		return "", fmt.Errorf("render: unsupported value type 0x%02x in Map renderer", byte(v.Type))
	}
}

// textContentString — конкатенация текстового содержимого nodes (аналог
// textContent из json.go, но без записи в буфер JSON-байтов).
func textContentString(nodes []binxml.Node, skipElements bool) (string, error) {
	var sb strings.Builder
	for _, n := range nodes {
		switch n.Kind {
		case binxml.KindText, binxml.KindCData:
			sb.WriteString(n.Text)
		case binxml.KindValue:
			s, err := valueTextString(n.Value)
			if err != nil {
				return "", err
			}
			sb.WriteString(s)
		case binxml.KindCharRef:
			ch := rune(n.CharRef)
			if utf8.ValidRune(ch) {
				sb.WriteRune(ch)
			} else {
				sb.WriteString("&#")
				sb.WriteString(strconv.FormatUint(uint64(n.CharRef), 10))
				sb.WriteByte(';')
			}
		case binxml.KindEntityRef:
			if resolved, ok := resolveEntity(n.Text); ok {
				sb.WriteString(resolved)
			} else {
				sb.WriteByte('&')
				sb.WriteString(n.Text)
				sb.WriteByte(';')
			}
		case binxml.KindPITarget, binxml.KindPIData:
			// ничего не выводим
		case binxml.KindPlaceholder:
			return "", unresolvedPlaceholder()
		case binxml.KindElement:
			if skipElements {
				continue
			}
			return "", fmt.Errorf("render: unexpected element node in text context")
		}
	}
	return sb.String(), nil
}

// tryAsNative — аналог tryAsNumber: nodes схлопываются ровно в одно
// числовое/bool значение без прочего содержимого.
func tryAsNative(nodes []binxml.Node, skipElements bool) (interface{}, bool, error) {
	var single *binxml.Node
	for i := range nodes {
		n := &nodes[i]
		switch n.Kind {
		case binxml.KindElement:
			if skipElements {
				continue
			}
			return nil, false, nil
		case binxml.KindText, binxml.KindCData:
			if skipElements {
				if n.Text != "" {
					return nil, false, nil
				}
			} else if single != nil || n.Text != "" {
				return nil, false, nil
			} else {
				single = n
			}
		case binxml.KindValue:
			if skipElements && n.Value.IsOptionalEmpty() {
				continue
			}
			if single != nil {
				return nil, false, nil
			}
			single = n
		case binxml.KindCharRef, binxml.KindEntityRef:
			return nil, false, nil
		case binxml.KindPITarget, binxml.KindPIData:
			if !skipElements && single != nil {
				return nil, false, nil
			}
			if !skipElements {
				single = n
			}
		case binxml.KindPlaceholder:
			if skipElements {
				return nil, false, unresolvedPlaceholder()
			}
			return nil, false, nil
		}
	}
	if single != nil && single.Kind == binxml.KindValue {
		v, ok := valueAsNative(single.Value)
		return v, ok, nil
	}
	return nil, false, nil
}

func contentAsNative(nodes []binxml.Node, skipElements bool) (interface{}, error) {
	v, ok, err := tryAsNative(nodes, skipElements)
	if err != nil {
		return nil, err
	}
	if ok {
		return v, nil
	}
	return textContentString(nodes, skipElements)
}

// tryLeafNative — аналог tryLeafValue.
func tryLeafNative(e *binxml.Element, emptyAsString bool) (interface{}, bool, error) {
	if e.HasElementChild || len(e.Children) != 1 {
		return nil, false, nil
	}
	var empty interface{}
	if emptyAsString {
		empty = ""
	}
	n := e.Children[0]
	switch n.Kind {
	case binxml.KindText, binxml.KindCData:
		if n.Text == "" {
			return empty, true, nil
		}
		return n.Text, true, nil
	case binxml.KindValue:
		if n.Value.IsOptionalEmpty() {
			return empty, true, nil
		}
		if v, ok := valueAsNative(n.Value); ok {
			return v, true, nil
		}
		s, err := valueTextString(n.Value)
		if err != nil {
			return nil, false, err
		}
		return s, true, nil
	default:
		return nil, false, nil
	}
}

// dataElementNative — аналог dataElementValue: пустое значение — "", а не nil.
func dataElementNative(e *binxml.Element) (interface{}, error) {
	v, wrote, err := tryLeafNative(e, true)
	if err != nil {
		return nil, err
	}
	if wrote {
		return v, nil
	}
	hasText, hasElementChild := contentLayout(e)
	if !hasText && !hasElementChild {
		return "", nil
	}
	if hasElementChild {
		return elementBodyNative(e, false, true, hasText)
	}
	return contentAsNative(e.Children, false)
}

// elementValueNative — аналог writeElementValue.
func elementValueNative(e *binxml.Element, childIsContainer bool) (interface{}, error) {
	attrs := e.Attrs
	if isEventIDElementName(e.Name) {
		attrs = nil
	}
	hasAnyAttr, hasAttrsText := attrFlags(attrs)
	if !hasAnyAttr {
		v, wrote, err := tryLeafNative(e, false)
		if err != nil {
			return nil, err
		}
		if wrote {
			return v, nil
		}
	}
	hasText, hasElementChild := contentLayout(e)

	if !hasElementChild && !hasText && !hasAttrsText {
		return nil, nil
	}
	if !hasElementChild && !hasAttrsText {
		return contentAsNative(e.Children, false)
	}
	return elementBodyNative(e, childIsContainer, false, hasText)
}

// elementBodyNative — аналог elementBody: атрибуты и дочерние элементы как
// ключи одного map[string]interface{}, с тем же выравниванием "Data" внутри
// EventData/UserData и суффиксами "_N" при коллизии имён.
func elementBodyNative(e *binxml.Element, inDataContainer, omitAttributes, hasText bool) (map[string]interface{}, error) {
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
	out := make(map[string]interface{})

	if !omitAttributes {
		for _, attr := range e.Attrs {
			if isSuppressedAttr(attr.Name) || !hasTextContent(attr.Value) {
				continue
			}
			suffix := nameCounts[attr.Name]
			nameCounts[attr.Name] = suffix + 1

			v, ok, err := tryAsNative(attr.Value, false)
			if err != nil {
				return nil, err
			}
			if !ok {
				s, err := textContentString(attr.Value, false)
				if err != nil {
					return nil, err
				}
				v = s
			}
			out[mapKey(attr.Name, suffix)] = v
		}
	}

	if hasText {
		v, err := contentAsNative(e.Children, true)
		if err != nil {
			return nil, err
		}
		out["#text"] = v
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
				key, err := textContentString(nameNodes, false)
				if err != nil {
					return nil, err
				}
				v, err := dataElementNative(child)
				if err != nil {
					return nil, err
				}
				out[key] = v
			} else if !positionalDataEmitted && positionalDataCount > 0 {
				positionalDataEmitted = true
				if positionalDataCount == 1 {
					v, err := dataElementNative(child)
					if err != nil {
						return nil, err
					}
					out["Data"] = map[string]interface{}{"#text": v}
				} else {
					arr := make([]interface{}, 0, positionalDataCount)
					for _, n2 := range e.Children {
						if n2.Kind != binxml.KindElement || !isDataElementName(n2.Element.Name) {
							continue
						}
						v, err := dataElementNative(n2.Element)
						if err != nil {
							return nil, err
						}
						arr = append(arr, v)
					}
					out["Data"] = map[string]interface{}{"#text": arr}
				}
			}
			continue
		}

		cname := child.Name
		suffix := nameCounts[cname]
		nameCounts[cname] = suffix + 1

		childIsContainer := isDataContainerName(cname)
		v, err := elementValueNative(child, childIsContainer)
		if err != nil {
			return nil, err
		}
		out[mapKey(cname, suffix)] = v
	}

	return out, nil
}
