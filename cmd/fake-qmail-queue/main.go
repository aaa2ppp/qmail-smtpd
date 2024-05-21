package main

import (
	"io"
	"log"
	"os"
)

func main() {
	var out0, out1 io.Writer

	out0 = os.Stderr
	if out_fn0, ok := os.LookupEnv("QQ_OUT0"); ok {
		fd, err := os.Create(out_fn0)
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		out0 = fd
	}

	out1 = out0
	if out_fn1, ok := os.LookupEnv("QQ_OUT1"); ok {
		fd, err := os.Create(out_fn1)
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		out1 = fd
	}

	if _, err := io.Copy(out0, os.Stdin); err != nil {
		log.Println("o0:", err)
	}

	if _, err := io.Copy(out1, os.Stdout); err != nil { // yes, reads from fd(1)
		log.Println("o1:", err)
	}
}
