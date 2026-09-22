package binxml

// arrayexpand.go реализует расширение MS-EVEN6 §3.1.4.7.5 ("Типы массивов"):
// когда разрешенная подстановка шаблона представляет собой значение массива с более чем
// одним элементом, *содержащий элемент* повторяется один раз для каждого элемента массива
// (вместо того, чтобы массив был представлен как один многозначный узел).

type arrayLocation struct {
	isAttr  bool
	attrIdx int
	nodeIdx int
}

type scalarReplacement struct {
	omit bool
	node Node
}

// expandArraySubstitutionsInElement рекурсивно расширяет каждую подстановку массива внутри
func expandArraySubstitutionsInElement(elem *Element) ([]*Element, error) {
	expandedOnce, err := expandFirstArrayInElement(elem)
	if err != nil {
		return nil, err
	}
	if expandedOnce == nil {
		return nil, nil
	}

	out := make([]*Element, 0, len(expandedOnce))
	for _, e := range expandedOnce {
		further, err := expandArraySubstitutionsInElement(e)
		if err != nil {
			return nil, err
		}
		if further != nil {
			out = append(out, further...)
		} else {
			out = append(out, e)
		}
	}
	return out, nil
}

func expandFirstArrayInElement(elem *Element) ([]*Element, error) {
	loc, arrayValue, length, found := findFirstArrayValue(elem)
	if !found || length <= 1 {
		return nil, nil
	}

	out := make([]*Element, 0, length)
	for idx := 0; idx < length; idx++ {
		repl, ok := scalarReplacementFromArrayValue(arrayValue, idx)
		if !ok {
			// Unknown/unsupported array representation: don't expand.
			return nil, nil
		}
		out = append(out, cloneElementWithReplacement(elem, loc, repl))
	}
	return out, nil
}

// Функция findFirstArrayValue сканирует дочерние элементы элемента (слева направо),
// затем значения его атрибутов (в порядке атрибутов, затем слева направо) для поиска первого узла KindValue,
// значение которого представляет собой массив с более чем одним элементом.
func findFirstArrayValue(elem *Element) (arrayLocation, *Value, int, bool) {
	for idx, n := range elem.Children {
		if n.Kind != KindValue {
			continue
		}
		length := n.Value.ExpandableArrayLen()
		if length <= 1 {
			continue
		}
		return arrayLocation{nodeIdx: idx}, n.Value, length, true
	}

	for aIdx, attr := range elem.Attrs {
		for nIdx, n := range attr.Value {
			if n.Kind != KindValue {
				continue
			}
			length := n.Value.ExpandableArrayLen()
			if length <= 1 {
				continue
			}
			return arrayLocation{isAttr: true, attrIdx: aIdx, nodeIdx: nIdx}, n.Value, length, true
		}
	}

	return arrayLocation{}, nil, 0, false
}

func cloneElementWithReplacement(elem *Element, loc arrayLocation, repl scalarReplacement) *Element {
	out := &Element{Name: elem.Name, HasElementChild: elem.HasElementChild}

	for aIdx, attr := range elem.Attrs {
		newAttr := Attr{Name: attr.Name}
		for nIdx, n := range attr.Value {
			if loc.isAttr && loc.attrIdx == aIdx && loc.nodeIdx == nIdx {
				if !repl.omit {
					newAttr.Value = append(newAttr.Value, repl.node)
				}
			} else {
				newAttr.Value = append(newAttr.Value, n)
			}
		}
		if len(newAttr.Value) > 0 {
			out.Attrs = append(out.Attrs, newAttr)
		}
	}

	for cIdx, n := range elem.Children {
		if !loc.isAttr && loc.nodeIdx == cIdx {
			if !repl.omit {
				out.Children = append(out.Children, repl.node)
			}
		} else {
			out.Children = append(out.Children, n)
		}
	}

	return out
}

func scalarReplacementFromArrayValue(v *Value, idx int) (scalarReplacement, bool) {
	item, ok := v.ArrayItemAsValue(idx)
	if !ok {
		return scalarReplacement{}, false
	}
	if item.Type == StringType {
		if item.Str.IsEmpty() {
			return scalarReplacement{omit: true}, true
		}
		return scalarReplacement{node: Node{Kind: KindText, Text: item.Str.String()}}, true
	}
	return scalarReplacement{node: Node{Kind: KindValue, Value: item}}, true
}
