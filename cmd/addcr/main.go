package main

import (
	"bufio"
	"io"
	"log"
	"os"
)

func main() {
	br := bufio.NewReader(os.Stdin)
	bw := bufio.NewWriter(os.Stdout)
	defer func() {
		if err := bw.Flush(); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		line, isPrefix, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		bw.Write(line)
		if !isPrefix {
			bw.WriteString("\r\n")
			err = bw.Flush()
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
