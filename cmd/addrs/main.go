package main

import (
	"fmt"
	"log"
	"net"

	"qmail-smtpd/internal/scan"
)

func main() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	for _, addr := range addrs {
		n, ip := scan.ScanIP(addr.String())
		fmt.Printf("%v %v ==> %d %v\n", addr.Network(), addr.String(), n, ip)
	}
}
