Для работы с CDB (Constant Database) в Go есть несколько пакетов с лицензиями BSD/MIT и без внешних зависимостей:

## Основные пакеты:

### 1. **[github.com/colinmarc/cdb](https://github.com/colinmarc/cdb)**
- **Лицензия**: MIT
- **Особенности**: Чистый Go, только чтение, очень легкий
- **Использование**:
```go
import "github.com/colinmarc/cdb"

db, err := cdb.Open("data.cdb")
defer db.Close()

value, err := db.Get([]byte("key"))
```

### 2. **[github.com/jbarham/cdb](https://github.com/jbarham/cdb)**
- **Лицензия**: BSD-style
- **Особенности**: Минималистичный, только чтение
- **Использование**:
```go
import "github.com/jbarham/gocdb"

db, err := gocdb.Open("data.cdb")
defer db.Close()

value, err := db.Get([]byte("key"))
```

### 3. **[github.com/bsm/ccdb](https://github.com/bsm/ccdb) ([cdb64](https://github.com/bsm/cdb64))**
- **Лицензия**: MIT
- **Особенности**: Простой API, только чтение
- **Использование**:
```go
import "github.com/bsm/cdb"

db, err := cdb.Open("data.cdb")
defer db.Close()

value, err := db.Get([]byte("key"))
```

## Рекомендация:

**github.com/colinmarc/cdb** - наиболее популярный и хорошо поддерживаемый вариант. Он:
- Имеет только 2 файла исходного кода
- Не имеет внешних зависимостей
- MIT лицензия
- Эффективная реализация только для чтения

Все эти пакеты соответствуют вашим требованиям: только чтение, легкие, без зависимостей, с BSD/MIT лицензиями.
