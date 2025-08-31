# ⚠️ Attention: It's a hobby project

**Do not use it in production.**  
This is a study, an experiment — a way to dig into qmail and rebuild it the way I’ve always wanted.

---

# qmail-smtpd (Go rewrite)

A respectful reimplementation of Dan Bernstein’s `qmail-smtpd` in Go.  
Not a clean-room rewrite — a **thoughtful evolution** with modern tooling.

## Goal

Replace the classic `tcpserver + qmail-smtpd` combo with a single, embeddable SMTP server that:
- Fits into the qmail ecosystem
- Needs no patches
- Is fully under my control

Eventually, it may run without `tcpserver`, `RELAYCLIENT`, or even `QMAILQUEUE`.  
But for now, I'm learning from djb's code — and building my own version.

## Why?

- I’m tired of patching `qmail-smtpd.c` by hand.
- I want full control: config, logging, auth, TLS.
- I want to understand how it *really* works — and make it mine.
- Go gives better testing, clarity, and safety.

## What I’m preserving

- The **spirit** of djb: minimal, direct, no magic.
- The **logic** of `blast` — it’s a masterpiece of state machine design.
- The **idea** of environment-driven execution (though I may change how it works).
- The **discipline** of small, composable parts.

## What I’m changing

- From C to Go — for clarity, safety, and tooling.
- From global state to explicit context.
- From patch hell to code I own.
- From mystery to understanding.

This is not about compatibility.  
It’s about **continuing a tradition** — not copying it.

## Status

**Initial development.**  
First commit was 11 months ago. I dropped it. Now I’m back.  
Might drop it again.  
This is a hobby.

---

**If you’re reading this, I’m still tinkering.**