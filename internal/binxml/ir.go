package binxml

// NodeKind определяет, какие поля узла имеют значение.
type NodeKind uint8

const (
	// KindElement: Node.Element содержит дочерний элемент (цель для вставки как для
	// экземпляров шаблонов, так и для вложенных фрагментов BinXML — отдельного типа узла "экземпляр шаблона" нет, см. treebuilder.go).
	KindElement NodeKind = iota
	// KindText: Node.Text содержит уже декодированное текстовое содержимое.
	KindText

	// KindValue: Node.Value содержит типизированное значение BinXML (все, что отличается от String/AnsiString: числа, GUID, SID, двоичные данные, время, массивы...).
	KindValue
	// KindEntityRef: Node.Text содержит имя сущности.
	KindEntityRef
	// KindCharRef: Node.CharRef содержит кодовую единицу UTF-16. Парсер
	// в настоящее время полностью отклоняет токены CharRef (соответствуя поведению "нереализованного токена" эталонной
	// реализации), поэтому этот тип
	// никогда фактически не создается сегодня; он существует для структурной четности и
	// чтобы IR/рендерер были готовы, если этот токен будет реализован позже.
	KindCharRef
	// KindCData: Node.Text содержит текст CDATA. Также никогда не создавался
	// сегодня (токены CDataSection отклоняются).
	KindCData
	// KindPITarget: Node.Text содержит имя целевого объекта инструкции обработки.
	KindPITarget
	// KindPIData: Node.Text содержит текстовые данные инструкции обработки.
	KindPIData
	// KindPlaceholder: Node.Placeholder содержит неразрешенный слот подстановки шаблона

	//Присутствует только в дереве определения шаблона
	// (BuildMode TemplateDefinition) до создания экземпляра; полностью
	// материализованное дерево записи никогда не должно его содержать.
	KindPlaceholder
)

// Заполнитель — это неразрешенный слот подстановки шаблона, который записывается во время
// разбора определения шаблона (см. BuildMode в treebuilder.go).
type Placeholder struct {
	// Индекс — это нулевой индекс в массиве значений подстановки экземпляра шаблона.
	Index int
	// Тип — это тип значения, объявленный в определении шаблона. Он

	// в настоящее время не используется во время разрешения (эталонная реализация
	// аналогично использует только *фактический* тип подстановки во время выполнения и
	// необязательный флаг при разрешении), но сохраняется для диагностики/обеспечения четности.
	Type ValueType
	// Optional обозначает "условную подстановку": когда полученное значение
	// пустое (см. Value.IsOptionalEmpty), заполнитель исчезает
	// полностью, вместо того чтобы создавать пустой узел.
	Optional bool
}

// Узел — это один из элементов внутри дочерних элементов элемента или списка значений атрибута.
type Node struct {
	Kind NodeKind

	Element *Element // KindElement

	// Текст содержит декодированный текст для KindText, KindEntityRef (имя сущности),
	// KindCData, KindPITarget и KindPIData.
	Text string

	// Value содержит типизированное значение BinXML для KindValue.
	Value *Value

	CharRef uint16 // KindCharRef: raw UTF-16 code unit

	Placeholder Placeholder // KindPlaceholder
}

// Attr — это один атрибут элемента: имя плюс последовательность узлов, содержащая его значение.
// (почти всегда один узел, но грамматика допускает больше).
type Attr struct {
	Name  string
	Value []Node
}

// Элемент представляет собой один элемент BinXML в дереве IR.
type Element struct {
	Name  string
	Attrs []Attr
	// В дочерних элементах содержится весь контент, не являющийся атрибутами: текст, значения, ссылки на сущности,

	// вложенные элементы, инструкции по обработке и (только в
	// неразрешенном дереве определения шаблона) заполнители.
	Children []Node
	// Функция HasElementChild записывает, содержит ли Children хотя бы один узел KindElement.
	HasElementChild bool
}

// valueToNode преобразует скалярное значение BinXmlValue в его представление Node.
func valueToNode(v *Value) (Node, error) {
	switch v.Type {
	case StringType:
		return Node{Kind: KindText, Text: v.Str.String()}, nil
	case AnsiStringType:
		return Node{Kind: KindText, Text: v.AStr}, nil
	case EvtXmlType, BinXmlType, EvtHandleType:
		return Node{}, &UnimplementedValueError{Type: v.Type}
	default:
		return Node{Kind: KindValue, Value: v}, nil
	}
}

// nodeNeedsArrayExpansion сообщает, является ли узел узлом типа KindValue, содержащим
// значение массива с более чем одним элементом — условие срабатывания для
// MS-EVEN6 §3.1.4.7.5 расширения подстановки массива (см. arrayexpand.go).
func nodeNeedsArrayExpansion(n Node) bool {
	if n.Kind != KindValue || n.Value == nil {
		return false
	}
	return n.Value.ExpandableArrayLen() > 1
}

// templateValue — это одна разрешенная подстановка экземпляра шаблона
type templateValue struct {
	element *Element
	value   Value
}

func (tv templateValue) isOptionalEmpty() bool {
	if tv.element != nil {
		return false
	}
	return tv.value.IsOptionalEmpty()
}

// elementTreeHasLiteralArray сообщает, имеет ли какой-либо элемент, достижимый из e
func elementTreeHasLiteralArray(e *Element) bool {
	for _, n := range e.Children {
		if nodeNeedsArrayExpansion(n) {
			return true
		}
		if n.Kind == KindElement && elementTreeHasLiteralArray(n.Element) {
			return true
		}
	}
	for _, a := range e.Attrs {
		for _, n := range a.Value {
			if nodeNeedsArrayExpansion(n) {
				return true
			}
		}
	}
	return false
}
