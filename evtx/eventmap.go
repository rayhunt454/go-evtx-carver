package evtx

// eventmap.go — EventMap, результат ParseFileToMap/ParseFileToMapStream:
// одна запись в виде generic-структуры (см. internal/render.Map) вместо
// JSON-байт, с удобным доступом к полям через
// Get/GetString/GetUint64/GetInt64/GetBool — для самодельных анализаторов
// логов, которым нужна выборка по значению поля (например, все записи с
// определённым EventID), а не готовый JSON.

import (
	"sort"
	"strconv"
	"strings"
)

// EventMap — одна запись, рендеренная в map[string]interface{} той же формы,
// что и JSON того же события (см. render.JSON): единственный ключ верхнего
// уровня — имя корневого элемента ("Event"), значение — вложенная структура
// из map[string]interface{}/[]interface{}/string/int64/uint64/bool/nil.
type EventMap map[string]interface{}

// Get ищет значение поля по имени key.
//
// Без точки в key — рекурсивный поиск первого совпадения по всему дереву
// записи, независимо от глубины вложенности: Get("EventID") находит его и
// внутри System, не требуя знать, что оно там лежит. Подходит для полей,
// которые в записи практически всегда одни (EventID, Channel, Computer,
// Provider, Level, TimeCreated) — если имя может встретиться в записи не
// один раз (например "Name" внутри разных EventData/Data), результат
// детерминирован (поиск по дочерним элементам одного уровня идёт в
// отсортированном по ключу порядке), но какое именно вхождение будет найдено
// первым — деталь реализации, а не гарантия по смыслу поля; в этом случае
// используйте путь с точками.
//
// С точкой в key — точный путь от корня записи, без ведущего имени корневого
// элемента: "System.EventID" вместо "Event.System.EventID". Путь идёт только
// через map[string]interface{} (не индексирует срезы) — для полей, которые
// не бывают массивами.
func (m EventMap) Get(key string) (interface{}, bool) {
	root, ok := eventRoot(m)
	if !ok {
		return nil, false
	}
	if strings.Contains(key, ".") {
		return getPath(root, strings.Split(key, "."))
	}
	return findDeep(root, key)
}

// GetString — как Get, но с приведением к string. Числовые/bool значения
// форматируются в текст (тем же способом, что и text-контент в JSON/Map);
// вложенные map/slice приведению не подлежат — false.
func (m EventMap) GetString(key string) (string, bool) {
	v, ok := m.Get(key)
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case int64:
		return strconv.FormatInt(x, 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case bool:
		return strconv.FormatBool(x), true
	default:
		return "", false
	}
}

// GetUint64 — как Get, но с приведением к uint64. Принимает и значение,
// уже хранящееся как int64 (если оно неотрицательное) — типы Int*/UInt* в
// BinXML часто выбираются не аналитиком, а форматом события, поэтому строгое
// требование "исходно был именно UInt* тип" было бы неудобной мелочью.
func (m EventMap) GetUint64(key string) (uint64, bool) {
	v, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case uint64:
		return x, true
	case int64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	default:
		return 0, false
	}
}

// GetInt64 — как Get, но с приведением к int64. Симметрично GetUint64:
// принимает и uint64, если он влезает в int64 без переполнения.
func (m EventMap) GetInt64(key string) (int64, bool) {
	v, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int64:
		return x, true
	case uint64:
		if x > 1<<63-1 {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

// GetBool — как Get, но с приведением к bool.
func (m EventMap) GetBool(key string) (bool, bool) {
	v, ok := m.Get(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// eventRoot возвращает содержимое единственного ключа верхнего уровня m
// (имени корневого элемента записи) — то, от чего строятся и глубокий
// поиск, и путь с точками.
func eventRoot(m EventMap) (interface{}, bool) {
	for _, v := range m {
		return v, true
	}
	return nil, false
}

// getPath проходит по node через map[string]interface{}, сегмент за
// сегментом. Срезы ([]interface{}) путь не индексирует — встретив срез
// раньше, чем кончились сегменты, возвращает "не найдено".
func getPath(node interface{}, segments []string) (interface{}, bool) {
	for _, seg := range segments {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil, false
		}
		node, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return node, true
}

// findDeep ищет key в node рекурсивно: сначала среди собственных ключей
// node (если это map), затем — в глубину по каждому дочернему
// map/срезу. Порядок обхода дочерних ключей одного уровня отсортирован по
// имени — так одинаковый вход всегда даёт одинаковый результат, даже если
// key неоднозначен (см. комментарий Get).
func findDeep(node interface{}, key string) (interface{}, bool) {
	switch v := node.(type) {
	case map[string]interface{}:
		if val, ok := v[key]; ok {
			return val, true
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if found, ok := findDeep(v[k], key); ok {
				return found, true
			}
		}
	case []interface{}:
		for _, item := range v {
			if found, ok := findDeep(item, key); ok {
				return found, true
			}
		}
	}
	return nil, false
}
