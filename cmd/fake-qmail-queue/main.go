package main

import (
	"io"
	"log"
	"os"
)

func main() {
	var o0, o1 io.Writer

	o0 = os.Stderr
	if ofn0, ok := os.LookupEnv("QQ_OUT0"); ok {
		fd, err := os.Create(ofn0)
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		o0 = fd
	}

	o1 = o0
	if ofn1, ok := os.LookupEnv("QQ_OUT1"); ok {
		fd, err := os.Create(ofn1)
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		o1 = fd
	}

	if _, err := io.Copy(o0, os.Stdin); err != nil {
		log.Println("o0:", err)
	}

	if _, err := io.Copy(o1, os.Stdout); err != nil { // yes, reads from fd(1)
		log.Println("o1:", err)
	}
}
