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

// GetByIP простой поиск по IP и префиксам IP, если поиск по хосту не нужен
func (r *Rules) GetByIP(ip string) (Result, error) {
	return r.GetByIPHost(ip, "")
}

// GetByIPHost
func (r *Rules) GetByIPHost(ip, host string) (Result, error) {
	var rule string
	err := r.db.Do(func(q Getter) error {
		var err error

		// ищем полный IP
		rule, err = q.Get(ip)
		if err == nil {
			return nil
		}
		if err != ErrNotFound {
			return err
		}

		// ищем полный хост если есть
		if host != "" {
			rule, err = q.Get("=" + host)
			if err == nil {
				return nil
			}
			if err != ErrNotFound {
				return err
			}
		}

		// ищем по префиксу IP (до последнего октета)
		rule, err = r.getByIPPrefixes(q, ip)
		if err == nil {
			return nil
		}
		if err != ErrNotFound {
			return err
		}

		// ищем по суффиксу FQDN (до пустой строки), если есть
		if host != "" {
			rule, err = r.getByHostSuffixes(q, host)
			if err == nil {
				return nil
			}
			if err != ErrNotFound {
				return err
			}

			rule, err = q.Get("=")
			if err == nil {
				return nil
			}
			if err != ErrNotFound {
				return err
			}
		}

		// ищем пустой ключ
		rule, err = q.Get("")
		return err
	})

	if err != nil {
		return Result{}, err
	}

	return ParseBinRule(rule)
}

func (r *Rules) getByIPPrefixes(q Getter, ip string) (string, error) {
	i := len(ip)
	for {
		i = strings.LastIndexByte(ip[:i], '.')
		if i == -1 {
			break
		}
		rule, err := q.Get(ip[:i+1])
		if err == nil {
			return rule, nil
		}
		if err != ErrNotFound {
			return "", err
		}
	}
	return "", ErrNotFound
}

func (r *Rules) getByHostSuffixes(q Getter, fqdn string) (string, error) {
	i := -1
	for {
		if j := strings.IndexByte(fqdn[i+1:], '.'); j == -1 {
			break
		} else {
			i += 1 + j
		}

		rule, err := q.Get("=" + fqdn[i:])
		if err == nil {
			return rule, nil
		}
		if err != ErrNotFound {
			return "", err
		}
	}
	return "", ErrNotFound
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
