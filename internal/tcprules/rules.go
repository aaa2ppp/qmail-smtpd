package tcprules

import (
	"errors"
	"io"
	"strings"
)

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrNotFound      = io.EOF
)

type Result struct {
	Allow bool
	Env   map[string]string
}

type DB interface {
	Do(func(Getter) error) error
}

type Getter interface {
	Get(key string) (string, error)
}

type Rules struct {
	db DB
}

func New(db DB) *Rules {
	return &Rules{db}
}

func (r *Rules) GetByIP(ip string) (Result, error) {
	// ищем от полного IP до пустой строки
	// "192.168.1.100" -> "192.168.1." -> "192.168." -> "192." -> ""

	var rule string
	err := r.db.Do(func(q Getter) error {
		for i := len(ip) - 1; ; {
			key := ip[:i+1]
			var err error
			rule, err = q.Get(key)
			if err != nil && err != ErrNotFound {
				return err
			}
			if err == nil {
				return nil
			}
			if i == -1 {
				break
			}
			i = strings.LastIndexByte(ip[:i], '.')
		}
		return ErrNotFound
	})

	if err != nil {
		return Result{}, err
	}

	return ParseBinRule(rule)
}

func ParseBinRule(data string) (Result, error) {
	// while ((next0 = byte_chr(data,datalen,0)) < datalen) {
	// 	switch(data[0]) {
	// 	case 'D':
	// 		flagdeny = 1;
	// 		break;
	// 	case '+':
	// 		split = str_chr(data + 1,'=');
	// 		if (data[1 + split] == '=') {
	// 			data[1 + split] = 0;
	// 			env(data + 1,data + 1 + split + 1);
	// 		}
	// 		break;
	// 	}
	// 	++next0;
	// 	data += next0; datalen -= next0;
	// }

	env := map[string]string{}

	for len(data) > 0 {
		next0 := strings.IndexByte(data, 0)
		if next0 == -1 {
			return Result{}, ErrInvalidFormat
		}

		chunk := data[:next0]
		data = data[next0+1:]

		if len(chunk) == 0 {
			continue
		}

		switch chunk[0] {
		case 'D':
			return Result{Allow: false}, nil
		case '+':
			chunk = chunk[1:]
			split := strings.IndexByte(chunk, '=')
			if split == -1 {
				return Result{}, ErrInvalidFormat
			}
			key := string(chunk[:split])
			value := string(chunk[split+1:])
			env[key] = value
		default:
			return Result{}, ErrInvalidFormat
		}
	}

	return Result{Allow: true, Env: env}, nil
}
