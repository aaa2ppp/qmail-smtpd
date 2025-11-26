package filters

import (
	"context"
	"log/slog"
	"strings"

	"qmail-smtpd/internal/cdb"
)

// int rcpthosts(buf,len)
// char *buf;
// int len;
// {
//   int j;

//   if (flagrh != 1) return 1;

//   j = byte_rchr(buf,len,'@');
//   if (j >= len) return 1; /* presumably envnoathost is acceptable */

//   ++j; buf += j; len -= j;

//   if (!stralloc_copyb(&host,buf,len)) return -1;
//   buf = host.s;
//   case_lowerb(buf,len);

//   for (j = 0;j < len;++j)
//     if (!j || (buf[j] == '.'))
//       if (constmap(&maprh,buf + j,len - j)) return 1;

//   if (fdmrh != -1) {
//     uint32 dlen;
//     int r;

//     for (j = 0;j < len;++j)
//       if (!j || (buf[j] == '.')) {
// 	r = cdb_seek(fdmrh,buf + j,len - j,&dlen);
// 	if (r) return r;
//       }
//   }

//   return 0;
// }

type RcptHostsDB interface {
	Do(func(RcptHostsFinder) error) error
}

type RcptHostsFinder interface {
	Find(key string) error
}

type RcptHosts struct {
	cmap ConstMap
	cdb  RcptHostsDB
}

func NewRcptHosts(cmap ConstMap, db RcptHostsDB) *RcptHosts {
	return &RcptHosts{cmap: cmap, cdb: db}
}

func (rh *RcptHosts) Match(_ context.Context, addr string) bool {
	j := strings.LastIndexByte(addr, '@')
	if j == -1 {
		// envnoathost is acceptable - как в оригинале
		return true
	}

	domain := strings.ToLower(addr[j+1:])

	// Сначала проверяем constmap
	if rh.cmap != nil {
		// Проверяем domain.com, .domain.com, .com и т.д.
		for i := 0; i < len(domain); i++ {
			if i == 0 || domain[i] == '.' {
				if rh.cmap.Contains(domain[i:]) {
					return true
				}
			}
		}
	}

	// Затем проверяем CDB
	if rh.cdb != nil {
		found := false
		err := rh.cdb.Do(func(f RcptHostsFinder) error {
			for i := 0; i < len(domain); i++ {
				if i == 0 || domain[i] == '.' {
					if err := f.Find(domain[i:]); err == nil {
						found = true
						return nil // нашли - выходим
					} else if err != cdb.ErrNotFound {
						return err // реальная ошибка
					}
					// ErrNotFound - продолжаем поиск
				}
			}
			return cdb.ErrNotFound
		})

		if err != nil && err != cdb.ErrNotFound {
			slog.Error("RcptHosts.Match: CDB lookup failed", "error", err, "domain", domain)
			// При ошибке CDB считаем что не нашли (fail-closed)
			return false
		}

		return found
	}

	return false
}
