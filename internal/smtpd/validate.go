package smtpd

func mailValidate(ss *session, addr string) (allow bool, reason string) {
	if ss.badMailFrom != nil && ss.badMailFrom.Match(ss.ctx, addr) {
		return false, "553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)"
	}
	return true, ""
}

func rcptValidate(ss *session, addr string) (allow bool, reason string) {
	if !ss.openRelay && !ss.state.relayClient {
		if ss.rcptHosts == nil || !ss.rcptHosts.Match(ss.ctx, addr) {
			return false, "553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)"
		}
	}

	if ss.noMailbox != nil && ss.noMailbox.Match(ss.ctx, addr) {
		return false, "553 mailbox does not exist (#5.1.1)"
	}

	return true, ""
}
