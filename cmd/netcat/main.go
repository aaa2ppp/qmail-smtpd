package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

var (
	addr    = flag.String("addr", "", "address to dial (required)")
	timeout = flag.Duration("t", 0, "connection timeout")
)

func main() {
	flag.Parse()
	if *addr == "" {
		fmt.Fprintf(os.Stderr, "address required\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan int)
	go func() {
		defer close(done)
		if _, err := io.Copy(os.Stdout, conn); err != nil {
			log.Print(err)
			done <- 1
		}
	}()

	if _, err := io.Copy(conn, os.Stdin); err != nil {
		log.Fatal(err)
	}

	if *timeout == 0 {
		os.Exit(<-done)
	}

	select {
	case code := <-done:
		os.Exit(code)
	case <-time.After(*timeout):
		log.Print("Timeout waiting for response")
		os.Exit(1)
	}
}
