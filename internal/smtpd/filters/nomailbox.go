package filters

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"qmail-smtpd/internal/logger"
)

type Resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

type NoMailbox struct {
	hosts       ConstMap
	at          string
	localIPHost string
	resolver    Resolver
}

func NewNoMailBox(hosts ConstMap, atZone string, localIPHost string, resolver Resolver) *NoMailbox {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &NoMailbox{
		hosts:       hosts,
		at:          "." + atZone + ".",
		localIPHost: localIPHost,
		resolver:    resolver,
	}
}

func (mbx *NoMailbox) parseAddr(addr string) (mailbox, domain string) {
	if j := strings.LastIndexByte(addr, '@'); j == -1 {
		// Локальный адрес без @
		mailbox = addr
		domain = mbx.localIPHost
	} else {
		mailbox = addr[:j]
		domain = addr[j+1:]
	}
	return mailbox, domain
}

func (mbx *NoMailbox) Match(ctx context.Context, addr string) bool {
	if addr == "" {
		logger.FromContext(ctx).Error("NoMailbox.Match: empty address")
		return true // пустой адрес - отклоняем
	}

	if mbx.hosts == nil {
		return false // нет конфигурации - пропускаем проверку
	}

	// Парсим адрес
	mailbox, domain := mbx.parseAddr(addr)

	// Проверяем, применяется ли к этому домену наша проверка
	if !mbx.hosts.Contains(domain) {
		return false // не наш домен - пропускаем
	}

	// Строим FQDN для DNS lookup
	fqdn := mailbox + mbx.at + domain

	// DNS lookup с таймаутом
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := mbx.resolver.LookupIP(ctx, "ip", fqdn)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			// DNS record не найден - ящика нет!
			logger.FromContext(ctx).Debug("NoMailbox: record not found", "fqdn", fqdn)
			return true
		}
		// Другие ошибки DNS (таймаут, сервер недоступен) - пропускаем проверку
		logger.FromContext(ctx).Warn("NoMailbox: DNS lookup failed", "fqdn", fqdn, "error", err)
		return false
	}

	// DNS record найден - ящик может существовать
	return false
}
