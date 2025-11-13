# tcprules CLI Tool

This tool provides functionality for creating and dumping CDB databases based on the `tcprules` format.

## Usage

```sh
tcprules [OPTIONS] [CDB_FILE] [TMP_FILE]
```

## Options

*   `-dump`: Output the dump of the CDB database to stdout. When this flag is set, `CDB_FILE` is read (or stdin if `-` is specified) and its contents are printed in the readable `tcprules` format. The `TMP_FILE` argument is not used in dump mode.
*   `-size N`: Specify the buffer size for reading (critical for dumping large records in dump mode, or for creating large CDBs). Default is 4K for dump, 64K for create.
*   `-h`, `-help`: Show this help message.

## Examples

```sh
# Create CDB from rules
tcprules rules.cdb rules.tmp < rules.txt
tcprules -size=65536 rules.cdb rules.tmp < large_rules.txt

# Dump CDB to readable format
tcprules -dump rules.cdb
tcprules -dump -size=65536 rules.cdb
tcprules -dump - < rules.cdb          # Read from stdin
```

## Notes

*   The `-dump` flag is an *extension* provided by this implementation for convenience. The original `tcprules` utility might not have this feature.
*   The dump format produced by `-dump` aims to be compatible with the input format described in the `tcprules` format documentation (e.g., `internal/tcprules/PARSER.md`), but see the `selectQuote` behavior below.
*   **Current Limitation (Dump):** The dumping process uses `selectQuote` to choose delimiters for environment variable values. If a value contains *all* standard printable ASCII delimiters (`'`, `"`, `` ` ``, `!` to `~`), the dump will fail with an error. This is a current limitation for dumping and differs slightly from a potential *input* format that could use escaping (see `selectQuote` code comments).
