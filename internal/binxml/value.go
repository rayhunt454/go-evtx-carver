package binxml

import (
	"fmt"
	"math"
	"time"

	"github.com/rayhunt454/go-evtx-carver/internal/bytesutil"
	"github.com/rayhunt454/go-evtx-carver/internal/winenc"
)

// ValueType — тег типа значения BinXML (MS-EVEN6 §2.2.4.17 "Variant Type").
type ValueType uint8

const (
	NullType   ValueType = 0x00
	StringType ValueType = 0x01
	// AnsiStringType декодируется настроенной ANSI-кодировкой (по умолчанию windows-1252).
	AnsiStringType ValueType = 0x02
	Int8Type       ValueType = 0x03
	UInt8Type      ValueType = 0x04
	Int16Type      ValueType = 0x05
	UInt16Type     ValueType = 0x06
	Int32Type      ValueType = 0x07
	UInt32Type     ValueType = 0x08
	Int64Type      ValueType = 0x09
	UInt64Type     ValueType = 0x0a
	Real32Type     ValueType = 0x0b
	Real64Type     ValueType = 0x0c
	BoolType       ValueType = 0x0d
	BinaryType     ValueType = 0x0e
	GuidType       ValueType = 0x0f
	SizeTType      ValueType = 0x10
	FileTimeType   ValueType = 0x11
	SysTimeType    ValueType = 0x12
	SidType        ValueType = 0x13
	HexInt32Type   ValueType = 0x14
	HexInt64Type   ValueType = 0x15
	EvtHandleType  ValueType = 0x20
	// BinXmlType — вложенный фрагмент BinXML: парсер встраивает его в дерево
	// как обычные дочерние элементы, а не хранит как значение.
	BinXmlType ValueType = 0x21
	EvtXmlType ValueType = 0x23

	StringArrayType     ValueType = 0x81
	AnsiStringArrayType ValueType = 0x82
	Int8ArrayType       ValueType = 0x83
	UInt8ArrayType      ValueType = 0x84
	Int16ArrayType      ValueType = 0x85
	UInt16ArrayType     ValueType = 0x86
	Int32ArrayType      ValueType = 0x87
	UInt32ArrayType     ValueType = 0x88
	Int64ArrayType      ValueType = 0x89
	UInt64ArrayType     ValueType = 0x8a
	Real32ArrayType     ValueType = 0x8b
	Real64ArrayType     ValueType = 0x8c
	BoolArrayType       ValueType = 0x8d
	BinaryArrayType     ValueType = 0x8e
	GuidArrayType       ValueType = 0x8f
	SizeTArrayType      ValueType = 0x90
	FileTimeArrayType   ValueType = 0x91
	SysTimeArrayType    ValueType = 0x92
	SidArrayType        ValueType = 0x93
	HexInt32ArrayType   ValueType = 0x94
	HexInt64ArrayType   ValueType = 0x95
)

// String возвращает короткое имя типа (без суффикса Type) — например,
// "UInt16" или "HexInt32Array". Пригодно как подсказка при карвинге по
// записям: позиция значения в списке подстановок сама по себе имени поля не
// даёт, но тип сужает круг кандидатов (например, EventID почти всегда
// UInt16).
func (t ValueType) String() string {
	switch t {
	case NullType:
		return "Null"
	case StringType:
		return "String"
	case AnsiStringType:
		return "AnsiString"
	case Int8Type:
		return "Int8"
	case UInt8Type:
		return "UInt8"
	case Int16Type:
		return "Int16"
	case UInt16Type:
		return "UInt16"
	case Int32Type:
		return "Int32"
	case UInt32Type:
		return "UInt32"
	case Int64Type:
		return "Int64"
	case UInt64Type:
		return "UInt64"
	case Real32Type:
		return "Real32"
	case Real64Type:
		return "Real64"
	case BoolType:
		return "Bool"
	case BinaryType:
		return "Binary"
	case GuidType:
		return "GUID"
	case SizeTType:
		return "SizeT"
	case FileTimeType:
		return "FileTime"
	case SysTimeType:
		return "SysTime"
	case SidType:
		return "SID"
	case HexInt32Type:
		return "HexInt32"
	case HexInt64Type:
		return "HexInt64"
	case EvtHandleType:
		return "EvtHandle"
	case BinXmlType:
		return "BinXml"
	case EvtXmlType:
		return "EvtXml"
	case StringArrayType:
		return "StringArray"
	case AnsiStringArrayType:
		return "AnsiStringArray"
	case Int8ArrayType:
		return "Int8Array"
	case UInt8ArrayType:
		return "UInt8Array"
	case Int16ArrayType:
		return "Int16Array"
	case UInt16ArrayType:
		return "UInt16Array"
	case Int32ArrayType:
		return "Int32Array"
	case UInt32ArrayType:
		return "UInt32Array"
	case Int64ArrayType:
		return "Int64Array"
	case UInt64ArrayType:
		return "UInt64Array"
	case Real32ArrayType:
		return "Real32Array"
	case Real64ArrayType:
		return "Real64Array"
	case BoolArrayType:
		return "BoolArray"
	case BinaryArrayType:
		return "BinaryArray"
	case GuidArrayType:
		return "GuidArray"
	case SizeTArrayType:
		return "SizeTArray"
	case FileTimeArrayType:
		return "FileTimeArray"
	case SysTimeArrayType:
		return "SysTimeArray"
	case SidArrayType:
		return "SidArray"
	case HexInt32ArrayType:
		return "HexInt32Array"
	case HexInt64ArrayType:
		return "HexInt64Array"
	default:
		return fmt.Sprintf("Unknown(0x%02x)", uint8(t))
	}
}

// validValueType сообщает, является ли b известным тегом ValueType.
func validValueType(b byte) (ValueType, bool) {
	switch ValueType(b) {
	case NullType, StringType, AnsiStringType, Int8Type, UInt8Type, Int16Type, UInt16Type,
		Int32Type, UInt32Type, Int64Type, UInt64Type, Real32Type, Real64Type, BoolType,
		BinaryType, GuidType, SizeTType, FileTimeType, SysTimeType, SidType, HexInt32Type,
		HexInt64Type, EvtHandleType, BinXmlType, EvtXmlType,
		StringArrayType, AnsiStringArrayType, Int8ArrayType, UInt8ArrayType, Int16ArrayType,
		UInt16ArrayType, Int32ArrayType, UInt32ArrayType, Int64ArrayType, UInt64ArrayType,
		Real32ArrayType, Real64ArrayType, BoolArrayType, BinaryArrayType, GuidArrayType,
		SizeTArrayType, FileTimeArrayType, SysTimeArrayType, SidArrayType, HexInt32ArrayType,
		HexInt64ArrayType:
		return ValueType(b), true
	default:
		return 0, false
	}
}

// Value — декодированное значение BinXML: размеченное объединение в виде
// плоской структуры (один набор полей на Type) вместо интерфейса, чтобы
// скалярные значения не требовали аллокации на кучу.
//
// Целые числа при разборе всегда расширяются до I64/U64 — исходная разрядность
// (8/16/32/64 бит) на выходе (текст или классификацию int/bool) не влияет, так
// что хранить её незачем.
type Value struct {
	Type ValueType

	I64  int64   // Int8/16/32/64Type
	U64  uint64  // UInt8/16/32/64Type, HexInt32/64Type (величина; формат выбирает Type)
	F32  float32 // Real32Type
	F64  float64 // Real64Type
	Bool bool    // BoolType

	Str  Utf16Slice // StringType
	AStr string     // AnsiStringType (уже декодирован в UTF-8)
	Bin  []byte     // BinaryType
	Guid [16]byte   // GuidType
	Time time.Time  // FileTimeType, SysTimeType
	Sid  []byte     // SidType (сырые байты; форматируется по требованию)

	// BinXML — сырой payload вложенного фрагмента для BinXmlType. Разбор
	// записей/шаблонов обрабатывает этот тип особо и встраивает уже
	// разобранный фрагмент в дерево раньше, так что здесь он почти не нужен.
	BinXML       []byte
	BinXMLOffset int // абсолютное смещение BinXML в данных чанка

	StrArray  []Utf16Slice // StringArrayType
	I64Array  []int64      // Int8/16/32/64ArrayType (расширено; см. I64)
	U64Array  []uint64     // UInt8/16/32/64Array, HexInt32/64Array (расширено; формат выбирает Type)
	F32Array  []float32    // Real32ArrayType
	F64Array  []float64    // Real64ArrayType
	BoolArray []bool       // BoolArrayType
	GuidArray [][16]byte   // GuidArrayType
	TimeArray []time.Time  // FileTime/SysTimeArrayType
	SidArray  [][]byte     // SidArrayType
}

// IsOptionalEmpty сообщает, считается ли значение "пустым": пустая
// необязательная подстановка шаблона опускается целиком, а элемент/атрибут
// с единственным пустым значением рендерится как null, а не как текст.
func (v *Value) IsOptionalEmpty() bool {
	switch v.Type {
	case NullType:
		return true
	case StringType:
		return v.Str.IsEmpty()
	case AnsiStringType:
		return v.AStr == ""
	case BinaryType, BinXmlType:
		return len(v.Bin) == 0 && len(v.BinXML) == 0
	case StringArrayType:
		return len(v.StrArray) == 0
	case Int8ArrayType, Int16ArrayType, Int32ArrayType, Int64ArrayType:
		return len(v.I64Array) == 0
	case UInt8ArrayType, UInt16ArrayType, UInt32ArrayType, UInt64ArrayType,
		HexInt32ArrayType, HexInt64ArrayType:
		return len(v.U64Array) == 0
	case Real32ArrayType:
		return len(v.F32Array) == 0
	case Real64ArrayType:
		return len(v.F64Array) == 0
	case BoolArrayType:
		return len(v.BoolArray) == 0
	case GuidArrayType:
		return len(v.GuidArray) == 0
	case FileTimeArrayType, SysTimeArrayType:
		return len(v.TimeArray) == 0
	case SidArrayType:
		return len(v.SidArray) == 0
	default:
		return false
	}
}

// ExpandableArrayLen возвращает число элементов массива для раскрытия по
// MS-EVEN6 §3.1.4.7.5, или -1, если v — не раскрываемый тип массива.
func (v *Value) ExpandableArrayLen() int {
	switch v.Type {
	case StringArrayType:
		return len(v.StrArray)
	case Int8ArrayType, Int16ArrayType, Int32ArrayType, Int64ArrayType:
		return len(v.I64Array)
	case UInt8ArrayType, UInt16ArrayType, UInt32ArrayType, UInt64ArrayType,
		HexInt32ArrayType, HexInt64ArrayType:
		return len(v.U64Array)
	case Real32ArrayType:
		return len(v.F32Array)
	case Real64ArrayType:
		return len(v.F64Array)
	case BoolArrayType:
		return len(v.BoolArray)
	case GuidArrayType:
		return len(v.GuidArray)
	case FileTimeArrayType, SysTimeArrayType:
		return len(v.TimeArray)
	case SidArrayType:
		return len(v.SidArray)
	default:
		return -1
	}
}

// scalarType возвращает скалярный ValueType, соответствующий типу массива t
// (например Int32ArrayType -> Int32Type).
func scalarType(t ValueType) ValueType {
	switch t {
	case StringArrayType:
		return StringType
	case Int8ArrayType:
		return Int8Type
	case Int16ArrayType:
		return Int16Type
	case Int32ArrayType:
		return Int32Type
	case Int64ArrayType:
		return Int64Type
	case UInt8ArrayType:
		return UInt8Type
	case UInt16ArrayType:
		return UInt16Type
	case UInt32ArrayType:
		return UInt32Type
	case UInt64ArrayType:
		return UInt64Type
	case HexInt32ArrayType:
		return HexInt32Type
	case HexInt64ArrayType:
		return HexInt64Type
	case Real32ArrayType:
		return Real32Type
	case Real64ArrayType:
		return Real64Type
	case BoolArrayType:
		return BoolType
	case GuidArrayType:
		return GuidType
	case FileTimeArrayType:
		return FileTimeType
	case SysTimeArrayType:
		return SysTimeType
	case SidArrayType:
		return SidType
	default:
		return NullType
	}
}

// ArrayItemAsValue возвращает элемент массива idx как отдельное скалярное
// Value, либо ok=false, если v не раскрываемый массив или idx вне диапазона.
func (v *Value) ArrayItemAsValue(idx int) (*Value, bool) {
	st := scalarType(v.Type)
	switch v.Type {
	case StringArrayType:
		if idx < 0 || idx >= len(v.StrArray) {
			return nil, false
		}
		return &Value{Type: st, Str: v.StrArray[idx]}, true
	case Int8ArrayType, Int16ArrayType, Int32ArrayType, Int64ArrayType:
		if idx < 0 || idx >= len(v.I64Array) {
			return nil, false
		}
		return &Value{Type: st, I64: v.I64Array[idx]}, true
	case UInt8ArrayType, UInt16ArrayType, UInt32ArrayType, UInt64ArrayType,
		HexInt32ArrayType, HexInt64ArrayType:
		if idx < 0 || idx >= len(v.U64Array) {
			return nil, false
		}
		return &Value{Type: st, U64: v.U64Array[idx]}, true
	case Real32ArrayType:
		if idx < 0 || idx >= len(v.F32Array) {
			return nil, false
		}
		return &Value{Type: st, F32: v.F32Array[idx]}, true
	case Real64ArrayType:
		if idx < 0 || idx >= len(v.F64Array) {
			return nil, false
		}
		return &Value{Type: st, F64: v.F64Array[idx]}, true
	case BoolArrayType:
		if idx < 0 || idx >= len(v.BoolArray) {
			return nil, false
		}
		return &Value{Type: st, Bool: v.BoolArray[idx]}, true
	case GuidArrayType:
		if idx < 0 || idx >= len(v.GuidArray) {
			return nil, false
		}
		return &Value{Type: st, Guid: v.GuidArray[idx]}, true
	case FileTimeArrayType, SysTimeArrayType:
		if idx < 0 || idx >= len(v.TimeArray) {
			return nil, false
		}
		return &Value{Type: st, Time: v.TimeArray[idx]}, true
	case SidArrayType:
		if idx < 0 || idx >= len(v.SidArray) {
			return nil, false
		}
		return &Value{Type: st, Sid: v.SidArray[idx]}, true
	default:
		return nil, false
	}
}

// UnimplementedValueError возвращается для форм значений, которые парсер
// намеренно не поддерживает.
type UnimplementedValueError struct {
	Type   ValueType
	Sized  bool
	Size   int
	Offset int
}

func (e *UnimplementedValueError) Error() string {
	return fmt.Sprintf("offset %d: unimplemented value variant 0x%02x (sized=%v size=%d)", e.Offset, byte(e.Type), e.Sized, e.Size)
}

// readSID читает один самоограниченный SID: 1 байт revision, 1 байт числа
// sub-authority, 6 байт authority, затем count*4 байт sub-authority.
func readSID(c *bytesutil.Cursor) ([]byte, error) {
	head, err := c.Peek(8, "sid")
	if err != nil {
		return nil, err
	}
	subCount := int(head[1])
	total := 8 + subCount*4
	return c.TakeBytes(total, "sid")
}

// readValue читает одно значение BinXML заданного типа из курсора. При
// sized=true size — заданная извне длина значения в байтах (из дескриптора,
// как в подстановках шаблона и элементах массива); при sized=false значение
// читается в собственном самоописывающем (обычно с префиксом длины) формате,
// как для прямого токена Value (0x05/0x45).
func readValue(c *bytesutil.Cursor, vt ValueType, sized bool, size int, ansiCodec func([]byte) string, chunkData []byte) (Value, error) {
	switch vt {
	case NullType:
		return Value{Type: NullType}, nil

	case StringType:
		if sized {
			if size == 0 {
				return Value{Type: StringType}, nil
			}
			if size%2 != 0 {
				return Value{}, fmt.Errorf("offset %d: sized UTF-16 string has odd byte length %d", c.Pos(), size)
			}
			s, err := readUtf16ByCharCount(c, size/2)
			if err != nil {
				return Value{}, err
			}
			return Value{Type: StringType, Str: s}, nil
		}
		s, err := readLenPrefixedUtf16(c, false)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: StringType, Str: s}, nil

	case AnsiStringType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		raw, err := c.TakeBytes(size, "ansi_string_value")
		if err != nil {
			return Value{}, err
		}
		filtered := make([]byte, 0, len(raw))
		for _, b := range raw {
			if b != 0 {
				filtered = append(filtered, b)
			}
		}
		return Value{Type: AnsiStringType, AStr: ansiCodec(filtered)}, nil

	case Int8Type:
		b, err := c.U8("i8")
		return Value{Type: Int8Type, I64: int64(int8(b))}, err
	case UInt8Type:
		b, err := c.U8("u8")
		return Value{Type: UInt8Type, U64: uint64(b)}, err
	case Int16Type:
		v, err := c.U16("i16")
		return Value{Type: Int16Type, I64: int64(int16(v))}, err
	case UInt16Type:
		v, err := c.U16("u16")
		return Value{Type: UInt16Type, U64: uint64(v)}, err
	case Int32Type:
		v, err := c.U32("i32")
		return Value{Type: Int32Type, I64: int64(int32(v))}, err
	case UInt32Type:
		v, err := c.U32("u32")
		return Value{Type: UInt32Type, U64: uint64(v)}, err
	case Int64Type:
		v, err := c.U64("i64")
		return Value{Type: Int64Type, I64: int64(v)}, err
	case UInt64Type:
		v, err := c.U64("u64")
		return Value{Type: UInt64Type, U64: v}, err

	case Real32Type:
		v, err := c.U32("f32")
		return Value{Type: Real32Type, F32: bitsToFloat32(v)}, err
	case Real64Type:
		v, err := c.U64("f64")
		return Value{Type: Real64Type, F64: bitsToFloat64(v)}, err

	case BoolType:
		raw, err := c.U32("bool")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: BoolType, Bool: int32(raw) != 0}, nil

	case GuidType:
		b, err := c.TakeBytes(16, "guid")
		if err != nil {
			return Value{}, err
		}
		var g [16]byte
		copy(g[:], b)
		return Value{Type: GuidType, Guid: g}, nil

	case SizeTType:
		if sized && size == 4 {
			v, err := c.U32("sizet32")
			return Value{Type: HexInt32Type, U64: uint64(v)}, err
		}
		if sized && size == 8 {
			v, err := c.U64("sizet64")
			return Value{Type: HexInt64Type, U64: v}, err
		}
		return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}

	case FileTimeType:
		raw, err := c.U64("filetime")
		if err != nil {
			return Value{}, err
		}
		t, err := winenc.FileTimeToTime(raw)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: FileTimeType, Time: t}, nil

	case SysTimeType:
		raw, err := c.TakeBytes(16, "systime")
		if err != nil {
			return Value{}, err
		}
		var b [16]byte
		copy(b[:], raw)
		t, err := winenc.SysTimeFromBytes(b)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: SysTimeType, Time: t}, nil

	case SidType:
		sid, err := readSID(c)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: SidType, Sid: sid}, nil

	case HexInt32Type:
		v, err := c.U32("hex32")
		return Value{Type: HexInt32Type, U64: uint64(v)}, err
	case HexInt64Type:
		v, err := c.U64("hex64")
		return Value{Type: HexInt64Type, U64: v}, err

	case BinXmlType:
		if sized {
			if size == 0 {
				return Value{Type: BinXmlType}, nil
			}
			start := c.Pos()
			b, err := c.TakeBytes(size, "binxml_payload")
			if err != nil {
				return Value{}, err
			}
			return Value{Type: BinXmlType, BinXML: b, BinXMLOffset: start}, nil
		}
		payloadLen, err := c.U16("binxml_payload_len")
		if err != nil {
			return Value{}, err
		}
		if payloadLen == 0 {
			return Value{Type: BinXmlType}, nil
		}
		start := c.Pos()
		b, err := c.TakeBytes(int(payloadLen), "binxml_payload")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: BinXmlType, BinXML: b, BinXMLOffset: start}, nil

	case BinaryType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		b, err := c.TakeBytes(size, "binary")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: BinaryType, Bin: b}, nil

	case StringArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		if size == 0 {
			return Value{Type: StringArrayType}, nil
		}
		end := c.Pos() + size
		var items []Utf16Slice
		for c.Pos() < end {
			s, err := readNullTerminatedUtf16(c)
			if err != nil {
				return Value{}, err
			}
			items = append(items, s)
		}
		return Value{Type: StringArrayType, StrArray: items}, nil

	case Int8ArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		b, err := c.TakeBytes(size, "i8_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]int64, len(b))
		for i, x := range b {
			items[i] = int64(int8(x))
		}
		return Value{Type: Int8ArrayType, I64Array: items}, nil

	case UInt8ArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		b, err := c.TakeBytes(size, "u8_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]uint64, len(b))
		for i, x := range b {
			items[i] = uint64(x)
		}
		return Value{Type: UInt8ArrayType, U64Array: items}, nil

	case Int16ArrayType:
		items, err := readAlignedI64Array(c, size, 2, "i16_array")
		return Value{Type: Int16ArrayType, I64Array: items}, err
	case Int32ArrayType:
		items, err := readAlignedI64Array(c, size, 4, "i32_array")
		return Value{Type: Int32ArrayType, I64Array: items}, err
	case Int64ArrayType:
		items, err := readAlignedI64Array(c, size, 8, "i64_array")
		return Value{Type: Int64ArrayType, I64Array: items}, err

	case UInt16ArrayType:
		items, err := readAlignedU64Array(c, size, 2, "u16_array")
		return Value{Type: UInt16ArrayType, U64Array: items}, err
	case UInt32ArrayType:
		items, err := readAlignedU64Array(c, size, 4, "u32_array")
		return Value{Type: UInt32ArrayType, U64Array: items}, err
	case UInt64ArrayType:
		items, err := readAlignedU64Array(c, size, 8, "u64_array")
		return Value{Type: UInt64ArrayType, U64Array: items}, err
	case HexInt32ArrayType:
		items, err := readAlignedU64Array(c, size, 4, "hex32_array")
		return Value{Type: HexInt32ArrayType, U64Array: items}, err
	case HexInt64ArrayType:
		items, err := readAlignedU64Array(c, size, 8, "hex64_array")
		return Value{Type: HexInt64ArrayType, U64Array: items}, err

	case Real32ArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		raw, err := readAlignedU64Array(c, size, 4, "f32_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]float32, len(raw))
		for i, x := range raw {
			items[i] = bitsToFloat32(uint32(x))
		}
		return Value{Type: Real32ArrayType, F32Array: items}, nil
	case Real64ArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		raw, err := readAlignedU64Array(c, size, 8, "f64_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]float64, len(raw))
		for i, x := range raw {
			items[i] = bitsToFloat64(x)
		}
		return Value{Type: Real64ArrayType, F64Array: items}, nil

	case BoolArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		raw, err := readAlignedU64Array(c, size, 4, "bool_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]bool, len(raw))
		for i, x := range raw {
			items[i] = int32(x) != 0
		}
		return Value{Type: BoolArrayType, BoolArray: items}, nil

	case GuidArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		if size%16 != 0 {
			return Value{}, fmt.Errorf("offset %d: misaligned guid_array (size=%d)", c.Pos(), size)
		}
		count := size / 16
		items := make([][16]byte, count)
		for i := range items {
			b, err := c.TakeBytes(16, "guid_array")
			if err != nil {
				return Value{}, err
			}
			copy(items[i][:], b)
		}
		return Value{Type: GuidArrayType, GuidArray: items}, nil

	case FileTimeArrayType:
		raw, err := readAlignedU64Array(c, size, 8, "filetime_array")
		if err != nil {
			return Value{}, err
		}
		items := make([]time.Time, len(raw))
		for i, x := range raw {
			t, err := winenc.FileTimeToTime(x)
			if err != nil {
				return Value{}, err
			}
			items[i] = t
		}
		return Value{Type: FileTimeArrayType, TimeArray: items}, nil

	case SysTimeArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		if size%16 != 0 {
			return Value{}, fmt.Errorf("offset %d: misaligned systime_array (size=%d)", c.Pos(), size)
		}
		count := size / 16
		items := make([]time.Time, count)
		for i := range items {
			b, err := c.TakeBytes(16, "systime_array")
			if err != nil {
				return Value{}, err
			}
			var arr [16]byte
			copy(arr[:], b)
			t, err := winenc.SysTimeFromBytes(arr)
			if err != nil {
				return Value{}, err
			}
			items[i] = t
		}
		return Value{Type: SysTimeArrayType, TimeArray: items}, nil

	case SidArrayType:
		if !sized {
			return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
		}
		if size == 0 {
			return Value{Type: SidArrayType}, nil
		}
		start := c.Pos()
		var items [][]byte
		for c.Pos()-start < size {
			s, err := readSID(c)
			if err != nil {
				return Value{}, err
			}
			items = append(items, s)
		}
		return Value{Type: SidArrayType, SidArray: items}, nil

	default:
		return Value{}, &UnimplementedValueError{Type: vt, Sized: sized, Size: size, Offset: c.Pos()}
	}
}

func readAlignedI64Array(c *bytesutil.Cursor, size, elemBytes int, what string) ([]int64, error) {
	if size == 0 {
		return nil, nil
	}
	if size%elemBytes != 0 {
		return nil, fmt.Errorf("offset %d: misaligned %s (size=%d, elem=%d)", c.Pos(), what, size, elemBytes)
	}
	count := size / elemBytes
	items := make([]int64, count)
	for i := range items {
		b, err := c.TakeBytes(elemBytes, what)
		if err != nil {
			return nil, err
		}
		items[i] = int64(leToU64(b))
		// Знаковое расширение из фактической разрядности элемента.
		switch elemBytes {
		case 2:
			items[i] = int64(int16(leToU64(b)))
		case 4:
			items[i] = int64(int32(leToU64(b)))
		case 8:
			items[i] = int64(leToU64(b))
		}
	}
	return items, nil
}

func readAlignedU64Array(c *bytesutil.Cursor, size, elemBytes int, what string) ([]uint64, error) {
	if size == 0 {
		return nil, nil
	}
	if size%elemBytes != 0 {
		return nil, fmt.Errorf("offset %d: misaligned %s (size=%d, elem=%d)", c.Pos(), what, size, elemBytes)
	}
	count := size / elemBytes
	items := make([]uint64, count)
	for i := range items {
		b, err := c.TakeBytes(elemBytes, what)
		if err != nil {
			return nil, err
		}
		items[i] = leToU64(b)
	}
	return items, nil
}

func leToU64(b []byte) uint64 {
	var v uint64
	for i := len(b) - 1; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v
}

func bitsToFloat32(bits uint32) float32 {
	return math.Float32frombits(bits)
}

func bitsToFloat64(bits uint64) float64 {
	return math.Float64frombits(bits)
}
