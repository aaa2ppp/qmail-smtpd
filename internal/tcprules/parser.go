package tcprules

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"unicode"
	"unsafe"
)

// Adder интерфейс для добавления скомпилированных правил
type Adder interface {
	// Add добавляет правило.
	//
	// ВНИМАНИЕ: key и data - сырые буферы, действительные только на время вызова.
	// Если данные нужны после возврата из Add - КЛОНИРУЙТЕ их!
	// Использование указателей после вызова приводит к неопределенному поведению.
	Add(key []byte, data []byte) error
}

// Parser парсит текстовые правила tcprules.
//
// ВНИМАНИЕ: Парсер оптимизирован для производительности и переиспользует буферы.
// Данные, передаваемые в Adder.Add, действительны только на время вызова.
// Если Adder нуждается в сохранении key или data после возврата из Add -
// он ДОЛЖЕН скопировать эти данные.
type Parser struct {
	adder Adder
	buf   *bytes.Buffer
}

func NewParser(adder Adder) *Parser {
	buf := new(bytes.Buffer)
	return &Parser{adder: adder, buf: buf}
}

// ParseLine парсит одну строку правила и вызывает метод Addr.Add(addr, rule) для каждой пары адрес-правило.
func (p *Parser) ParseLine(line []byte) error {
	line = bytes.TrimRight(line, " \t\r\n")

	pos := 0
	for len(line) > 0 && unicode.IsSpace(rune(line[0])) {
		pos++
		line = line[1:]
	}

	// Пропускаем пустые строки и комментарии
	if len(line) == 0 || line[0] == '#' {
		return nil
	}

	colon := bytes.IndexByte(line, ':')
	if colon == -1 {
		return fmt.Errorf("missing colon in rule")
	}

	address := line[:colon]
	addresses, err := p.expandAddressRange(address)
	if err != nil {
		return fmt.Errorf("expand address %s: %w", address, err)
	}

	pos += colon + 1
	line = line[colon+1:]

	if n, err := p.parseRule(p.buf, unsafeString(line)); err != nil {
		return fmt.Errorf("parse rule at position %d: %w", pos+n, err)
	}

	rule := p.buf.Bytes()

	for addr := range addresses {
		if err := p.adder.Add(addr, rule); err != nil {
			return fmt.Errorf("add rule for %s: %w", addr, err)
		}
	}

	p.buf.Reset()
	return nil
}

func unsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

const MaxAddressesInRange = 10000

func (p *Parser) expandAddressRange(address []byte) (iter.Seq[[]byte], error) {
	if len(address) == 0 || bytes.ContainsAny(address, "=@") {
		return func(yield func([]byte) bool) { yield(address) }, nil
	}

	parts := strings.Split(unsafeString(address), ".")

	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	ranges := make([][2]int, 0, len(parts))
	for _, part := range parts {
		hyphen := strings.IndexByte(part, '-')
		if hyphen == -1 {
			octet, err := strconv.Atoi(part)
			if err != nil || octet < 0 || octet > 255 {
				return nil, fmt.Errorf("invalid octet: %s", part)
			}
			ranges = append(ranges, [2]int{octet, octet})
		} else {
			start, err := strconv.Atoi(part[:hyphen])
			if err != nil || start < 0 || start > 255 {
				return nil, fmt.Errorf("invalid start range: %s", part[:hyphen])
			}
			end, err := strconv.Atoi(part[hyphen+1:])
			if err != nil || end < 0 || end > 255 {
				return nil, fmt.Errorf("invalid end range: %s", part[hyphen+1:])
			}
			if start > end {
				return nil, fmt.Errorf("invalid range: %d-%d", start, end)
			}
			ranges = append(ranges, [2]int{start, end})
		}
	}

	// Вычисляем общее количество комбинаций
	total := 1
	for _, r := range ranges {
		total *= (r[1] - r[0] + 1)
	}
	if total > MaxAddressesInRange {
		return nil, fmt.Errorf("too many addresses in range: %d", total)
	}

	// Генерируем все комбинации с помощью замыкания
	return func(yield func([]byte) bool) {
		var generate func(prefix []byte, depth int) bool

		generate = func(prefix []byte, depth int) bool {
			if depth == len(ranges) {
				if depth < 4 {
					prefix = append(prefix, '.')
				}
				return yield(prefix[1:])
			}

			prefix = append(prefix, '.')

			for octet := ranges[depth][0]; octet <= ranges[depth][1]; octet++ {
				nextPrefix := strconv.AppendInt(prefix, int64(octet), 10)
				if !generate(nextPrefix, depth+1) {
					return false
				}
			}
			return true
		}

		generate(make([]byte, 0, 16), 0)
	}, nil
}

// parseRule парсит правую часть правила после двоеточия
func (p *Parser) parseRule(buf *bytes.Buffer, rule string) (int, error) {
	if strings.HasPrefix(rule, "deny") {
		buf.Write([]byte("D\x00"))
		return 4, nil
	}

	if !strings.HasPrefix(rule, "allow") {
		return 0, errors.New("invalid action")
	}

	pos := 5
	rule = rule[pos:]

	for len(rule) > 0 {
		if rule[0] != ',' {
			return pos, errors.New("expected comma")
		}
		pos++
		rule = rule[1:]

		n, err := p.parseEnv(buf, rule)
		if err != nil {
			return pos + n, err
		}
		pos += n
		rule = rule[n:]
	}

	return pos, nil
}

func (p *Parser) parseEnv(buf *bytes.Buffer, rule string) (int, error) {
	equals := strings.IndexByte(rule, '=')
	if equals == -1 {
		return 0, errors.New("missing = in env var")
	}

	key := rule[:equals]
	if key == "" {
		return 0, errors.New("empty key in env var")
	}

	if !isValidEnvName(rule[:equals]) {
		return 0, fmt.Errorf("invalid env name: %s", rule[:equals])
	}

	buf.WriteByte('+')
	buf.WriteString(rule[:equals+1])

	pos := equals + 1
	rule = rule[equals+1:]

	n, err := p.parseQuotedValue(buf, rule)
	if err != nil {
		return pos + n, err
	}

	return pos + n, nil
}

// isValidEnvName проверяет валидность имени переменной окружения
func isValidEnvName(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Первый символ должен быть буквой или подчеркиванием
	first := name[0]
	if !(first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' || first == '_') {
		return false
	}

	// Остальные символы - буквы, цифры, подчеркивания
	for i := 1; i < len(name); i++ {
		ch := name[i]
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
			ch >= '0' && ch <= '9' || ch == '_') {
			return false
		}
	}

	return true
}

// parseQuotedValue парсит значения в кавычках/разделителях
func (p *Parser) parseQuotedValue(buf *bytes.Buffer, rule string) (int, error) {
	if len(rule) < 2 {
		return 0, errors.New("value too short")
	}

	// Первый символ - кавычка
	quoteChar := rule[0]
	pos := 1
	rule = rule[1:]

	close := strings.IndexByte(rule, quoteChar)
	if close == -1 {
		return pos, fmt.Errorf("expected quote char '%c'", quoteChar)
	}

	buf.WriteString(rule[:close])
	buf.WriteByte(0)
	return pos + close + 1, nil
}
