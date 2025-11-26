package auth

// Credentials интерфейс для всех типов учетных данных.
type Credentials interface {
	_IqqDhSClB7fNACMfauiox9pM()
}

type credentials struct{}

func (c credentials) _IqqDhSClB7fNACMfauiox9pM() {}

// CRAMCredentials содержит данные для CRAM-MD5 аутентификации
type CRAMCredentials struct {
	credentials
	Challenge string // вызов сервера
	Username  string // имя пользователя
	Hexdigest string // хеш-дайджест ответа клиента
}

// PlainCredentials содержит данные для PLAIN аутентификации
type PlainCredentials struct {
	credentials
	AuthZID string // identity to authorize as (обычно игнорируется)
	AuthCID string // identity to authenticate as
	Passwd  string // пароль
}

type Result struct {
	Username string
	Success  bool
	// TODO: add some metadata if necessary...
}
