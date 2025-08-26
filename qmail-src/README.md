# qmail-src

Automated fetching and unpacking of qmail source archives.

## Usage

Default (netqmail-1.06):
```bash
make all
```

Custom version:
```bash
make NAME=qmail-1.03 all
```

## Targets
- `fetch`     — download archive
- `check`     — verify checksum
- `unpack`    — extract source
- `all`       — fetch + check + unpack
- `clean-src` — remove source directories
- `clean-all` — remove all downloaded and generated files

Supported values for `NAME`: `qmail-1.03`, `netqmail-1.06`
