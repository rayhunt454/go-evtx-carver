package carve

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rayhunt454/go-evtx-carver/internal/binxml"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// maxValueRecursionDepth ограничивает глубину рекурсии formatValue во
// вложенные значения BinXmlType (значение шаблона может само быть другим
// экземпляром шаблона) — защита от патологического/повреждённого входа.
const maxValueRecursionDepth = 16

// formatValues рендерит каждое значение из vals как читаемый текст, в
// порядке объявления, без имени поля (имена — chunk-relative и здесь
// недоступны).
func formatValues(vals []binxml.Value, ansiCodec func([]byte) string) []string {
	out := make([]string, 0, len(vals))
	for i := range vals {
		out = append(out, formatValue(&vals[i], ansiCodec, 0))
	}
	return out
}

// valueTypeNames возвращает имя wire-типа (binxml.ValueType.String()) для
// каждого значения из vals, в том же порядке, что и formatValues — вместе
// они образуют пары (тип, значение). Имя поля так узнать нельзя (см.
// formatValues), но тип сужает круг кандидатов: например, EventID почти
// всегда закодирован как UInt16, и это позволяет быстро отличить его от
// соседних значений другого типа.
func valueTypeNames(vals []binxml.Value) []string {
	out := make([]string, len(vals))
	for i := range vals {
		out[i] = vals[i].Type.String()
	}
	return out
}

func formatValue(v *binxml.Value, ansiCodec func([]byte) string, depth int) string {
	switch v.Type {
	case binxml.NullType:
		return ""
	case binxml.StringType:
		return v.Str.String()
	case binxml.AnsiStringType:
		return v.AStr
	case binxml.Int8Type, binxml.Int16Type, binxml.Int32Type, binxml.Int64Type:
		return strconv.FormatInt(v.I64, 10)
	case binxml.UInt8Type, binxml.UInt16Type, binxml.UInt32Type, binxml.UInt64Type:
		return strconv.FormatUint(v.U64, 10)
	case binxml.Real32Type:
		return strconv.FormatFloat(float64(v.F32), 'g', -1, 32)
	case binxml.Real64Type:
		return strconv.FormatFloat(v.F64, 'g', -1, 64)
	case binxml.BoolType:
		return strconv.FormatBool(v.Bool)
	case binxml.BinaryType:
		return fmt.Sprintf("%X", v.Bin)
	case binxml.GuidType:
		return winenc.FormatGUID(v.Guid)
	case binxml.FileTimeType, binxml.SysTimeType:
		return v.Time.UTC().Format("2006-01-02T15:04:05.000000Z")
	case binxml.SidType:
		return winenc.FormatSID(v.Sid)
	case binxml.HexInt32Type, binxml.HexInt64Type:
		return "0x" + strconv.FormatUint(v.U64, 16)
	case binxml.BinXmlType:
		return formatNestedBinXML(v.BinXML, ansiCodec, depth)

	case binxml.StringArrayType:
		return joinArray(len(v.StrArray), func(i int) string { return v.StrArray[i].String() })
	case binxml.Int8ArrayType, binxml.Int16ArrayType, binxml.Int32ArrayType, binxml.Int64ArrayType:
		return joinArray(len(v.I64Array), func(i int) string { return strconv.FormatInt(v.I64Array[i], 10) })
	case binxml.UInt8ArrayType, binxml.UInt16ArrayType, binxml.UInt32ArrayType, binxml.UInt64ArrayType:
		return joinArray(len(v.U64Array), func(i int) string { return strconv.FormatUint(v.U64Array[i], 10) })
	case binxml.HexInt32ArrayType, binxml.HexInt64ArrayType:
		return joinArray(len(v.U64Array), func(i int) string { return "0x" + strconv.FormatUint(v.U64Array[i], 16) })
	case binxml.Real32ArrayType:
		return joinArray(len(v.F32Array), func(i int) string { return strconv.FormatFloat(float64(v.F32Array[i]), 'g', -1, 32) })
	case binxml.Real64ArrayType:
		return joinArray(len(v.F64Array), func(i int) string { return strconv.FormatFloat(v.F64Array[i], 'g', -1, 64) })
	case binxml.BoolArrayType:
		return joinArray(len(v.BoolArray), func(i int) string { return strconv.FormatBool(v.BoolArray[i]) })
	case binxml.GuidArrayType:
		return joinArray(len(v.GuidArray), func(i int) string { return winenc.FormatGUID(v.GuidArray[i]) })
	case binxml.SidArrayType:
		return joinArray(len(v.SidArray), func(i int) string { return winenc.FormatSID(v.SidArray[i]) })
	case binxml.FileTimeArrayType, binxml.SysTimeArrayType:
		return joinArray(len(v.TimeArray), func(i int) string { return v.TimeArray[i].UTC().Format("2006-01-02T15:04:05.000000Z") })

	default:
		return fmt.Sprintf("<unhandled value type 0x%02x>", byte(v.Type))
	}
}

func joinArray(n int, at func(i int) string) string {
	items := make([]string, n)
	for i := range items {
		items[i] = at(i)
	}
	return "[" + strings.Join(items, "; ") + "]"
}

// formatNestedBinXML форматирует payload вложенной подстановки BinXmlType:
// рекурсивно как ещё один standalone TemplateInstance, где возможно, иначе —
// сырой текстовый sweep как последний резерв.
func formatNestedBinXML(payload []byte, ansiCodec func([]byte) string, depth int) string {
	if len(payload) == 0 {
		return ""
	}
	if depth < maxValueRecursionDepth {
		if ti, err := binxml.ParseStandaloneTemplateInstance(payload, ansiCodec); err == nil {
			nested := make([]string, 0, len(ti.Values))
			for i := range ti.Values {
				nested = append(nested, formatValue(&ti.Values[i], ansiCodec, depth+1))
			}
			return "[" + strings.Join(nested, "; ") + "]"
		}
	}
	if strs := sweepUTF16Strings(payload, 4); len(strs) > 0 {
		return "{" + strings.Join(strs, "; ") + "}"
	}
	return fmt.Sprintf("<binxml %d bytes>", len(payload))
}
