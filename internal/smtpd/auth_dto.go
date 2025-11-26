package smtpd

import "qmail-smtpd/internal/auth"

type (
	Credentials      = auth.Credentials
	cramCredentials  = auth.CRAMCredentials
	plainCredentials = auth.PlainCredentials
	authResult       = auth.Result
)
