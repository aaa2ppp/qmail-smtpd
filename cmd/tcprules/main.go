package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unsafe"

	"qmail-smtpd/internal/cdb"
	"qmail-smtpd/internal/tcprules"
)

var (
	dumpFlag = flag.Bool("dump", false, "Output dump CDB database to stdout")
	bufSize  = flag.Int("size", 0, "Buffer size for reading, critical for dump of large records (default: 4K for dump, 64K for create)")
	helpFlag = flag.Bool("h", false, "Show this help message")
	helpLong = flag.Bool("help", false, "Show this help message")
)

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage: %s [OPTIONS] [CDB_FILE] [TMP_FILE]\n", os.Args[0])
	fmt.Fprintf(out, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(out, `

Examples:
  # Create CDB from rules
  %[1]s rules.cdb rules.tmp < rules.txt
  %[1]s -size=65536 rules.cdb rules.tmp < large_rules.txt

  # Dump CDB to readable format
  %[1]s -dump rules.cdb
  %[1]s -dump -size=65536 rules.cdb
  %[1]s -dump - < rules.cdb          # Read from stdin
`, os.Args[0])
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *helpFlag || *helpLong {
		usage()
		return
	}

	if *dumpFlag {
		// Определяем входной файл для дампа
		var inputFile string
		if len(flag.Args()) > 0 {
			inputFile = flag.Arg(0)
		} else {
			inputFile = "-" // по умолчанию stdin
		}

		if err := runDump(inputFile, *bufSize); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Режим создания CDB
	if len(flag.Args()) < 2 {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Error: CDB_FILE and TMP_FILE arguments required for create mode\n\n")
		usage()
		os.Exit(1)
	}

	cdbFileName := flag.Arg(0)
	tmpFileName := flag.Arg(1)

	if err := runMake(cdbFileName, tmpFileName, *bufSize); err != nil {
		log.Fatal(err)
	}
}

func runMake(cdbFileName, tmpFileName string, bufSize int) error {
	tmpFile, err := os.OpenFile(tmpFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	var success bool
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpFileName)
		}
	}()

	if err := make(tmpFile, os.Stdin, bufSize); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpFileName, cdbFileName); err != nil {
		return err
	}

	success = true
	return nil
}

func make(w cdb.MakeWriter, r io.Reader, bufSize int) error {
	maker, err := cdb.Make(w)
	if err != nil {
		return err
	}

	parser := tcprules.NewParser(maker)
	sc := bufio.NewScanner(r)
	if bufSize > bufio.MaxScanTokenSize {
		sc.Buffer(nil, bufSize)
	}
	lineNum := 0

	for sc.Scan() {
		lineNum++
		if err := parser.ParseLine(sc.Bytes()); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	if err := maker.Finish(); err != nil {
		return fmt.Errorf("finalize CDB: %w", err)
	}

	return nil
}

func runDump(inputFile string, bufSize int) error {
	var r io.Reader

	if inputFile == "-" {
		r = os.Stdin
	} else {
		file, err := os.Open(inputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		r = file
	}
	return dump(os.Stdout, r, bufSize)
}

func dump(w io.Writer, r io.Reader, bufSize int) error {
	if bufSize > 4096 {
		r = bufio.NewReaderSize(r, bufSize)
	}

	dump, err := cdb.Dump(r)
	if err != nil {
		return err
	}

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for it := range dump {
		if it.Err != nil {
			return it.Err
		}

		key, data := it.Key, it.Data

		res, err := tcprules.ParseBinRule(unsafeString(data))
		if err != nil {
			return err
		}

		bw.Write(key)
		bw.WriteByte(':')

		if res.Allow {
			bw.WriteString("allow")
		} else {
			bw.WriteString("deny")
		}

		for name, value := range res.Env {
			bw.WriteByte(',')
			bw.WriteString(name)
			bw.WriteByte('=')

			quote, err := selectQuote(value)
			if err != nil {
				return err
			}
			bw.WriteByte(quote)
			bw.WriteString(value)
			bw.WriteByte(quote)
		}

		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}

	return nil
}

func unsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func selectQuote(value string) (byte, error) {
	// Проверяем самые частые кавычки в первую очередь
	if strings.IndexByte(value, '\'') == -1 {
		return '\'', nil
	}
	if strings.IndexByte(value, '"') == -1 {
		return '"', nil
	}
	if strings.IndexByte(value, '`') == -1 {
		return '`', nil
	}

	// Если все основные кавычки заняты, ищем любой доступный символ
	for q := byte('!'); q <= '~'; q++ {
		if strings.IndexByte(value, q) == -1 {
			return q, nil
		}
	}

	return 0, fmt.Errorf("couldn't select a quotation mark for %q", value)
}
