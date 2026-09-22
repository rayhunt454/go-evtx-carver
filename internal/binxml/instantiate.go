package binxml

import "fmt"

// Функция cloneAndResolve выполняет глубокое клонирование элемента определения шаблона (который может содержать
// узлы Placeholder и вложенные элементы) в полностью разрешенный
// элемент, заменяя каждый Placeholder соответствующим значением экземпляра
// . Она возвращает клонированный элемент и указывает, требуется ли расширение массива MS-EVEN6 §3.1.4.7.5 для каких-либо его
// (после разрешения) дочерних элементов/значений атрибутов — вызывающая сторона (resolveNodeInto, для вложенного элемента; или
// точка входа при создании экземпляра шаблона, для корня) решает, что делать с
// этим флагом.
func cloneAndResolve(tplElem *Element, values []templateValue) (*Element, bool, error) {
	resolved := &Element{Name: tplElem.Name}
	if len(tplElem.Attrs) > 0 {
		resolved.Attrs = make([]Attr, 0, len(tplElem.Attrs))
	}
	if len(tplElem.Children) > 0 {

		resolved.Children = make([]Node, 0, len(tplElem.Children))
	}
	needsArrayExpansion := false

	for _, attr := range tplElem.Attrs {
		newAttr := Attr{Name: attr.Name}
		if len(attr.Value) > 0 {
			newAttr.Value = make([]Node, 0, len(attr.Value))
		}
		for _, node := range attr.Value {
			before := len(newAttr.Value)
			var err error
			newAttr.Value, err = resolveNodeInto(node, values, newAttr.Value)
			if err != nil {
				return nil, false, err
			}
			if !needsArrayExpansion {
				for _, n := range newAttr.Value[before:] {
					if nodeNeedsArrayExpansion(n) {
						needsArrayExpansion = true
						break
					}
				}
			}
		}
		if len(newAttr.Value) > 0 {
			resolved.Attrs = append(resolved.Attrs, newAttr)
		}
	}

	for _, node := range tplElem.Children {
		before := len(resolved.Children)
		var err error
		resolved.Children, err = resolveNodeInto(node, values, resolved.Children)
		if err != nil {
			return nil, false, err
		}
		for _, n := range resolved.Children[before:] {
			if !resolved.HasElementChild && n.Kind == KindElement {
				resolved.HasElementChild = true
			}
			if !needsArrayExpansion && nodeNeedsArrayExpansion(n) {
				needsArrayExpansion = true
			}
		}
	}

	return resolved, needsArrayExpansion, nil
}

// Функция resolveNodeInto разрешает один узел определения шаблона и добавляет результат в out.
func resolveNodeInto(node Node, values []templateValue, out []Node) ([]Node, error) {
	switch node.Kind {
	case KindPlaceholder:
		return resolvePlaceholderInto(node.Placeholder, values, out)
	case KindElement:
		cloned, needsArrayExpansion, err := cloneAndResolve(node.Element, values)
		if err != nil {
			return nil, err
		}
		if needsArrayExpansion {
			expanded, err := expandArraySubstitutionsInElement(cloned)
			if err != nil {
				return nil, err
			}
			if expanded != nil {
				for _, e := range expanded {
					out = append(out, Node{Kind: KindElement, Element: e})
				}
				return out, nil
			}
		}
		return append(out, Node{Kind: KindElement, Element: cloned}), nil
	default:

		return append(out, node), nil
	}
}

// Функция resolvePlaceholderInto разрешает один Placeholder относительно значений подстановки экземпляра и добавляет 0 или 1 узел в out.
func resolvePlaceholderInto(ph Placeholder, values []templateValue, out []Node) ([]Node, error) {
	if ph.Index < 0 || ph.Index >= len(values) {

		return out, nil
	}
	tv := &values[ph.Index]
	if ph.Optional && tv.isOptionalEmpty() {
		return out, nil
	}
	if tv.element != nil {
		return append(out, Node{Kind: KindElement, Element: tv.element}), nil
	}
	v := &tv.value
	if v.Type == EvtXmlType {
		return nil, fmt.Errorf("unimplemented: EvtXml value in template substitution")
	}
	if v.Type == BinXmlType {
		return nil, fmt.Errorf("unsupported BinXML value in template substitution")
	}
	node, err := valueToNode(v)
	if err != nil {
		return nil, err
	}
	return append(out, node), nil
}
