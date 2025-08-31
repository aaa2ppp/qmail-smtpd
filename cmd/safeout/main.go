package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"unicode"
	"unicode/utf8"
)

func main() {
	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		if unicode.IsPrint(rune(c)) || c == '\n' || c == '\r' {
			err = w.WriteByte(c)
		} else {
			_, err = w.WriteRune(utf8.RuneError)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
