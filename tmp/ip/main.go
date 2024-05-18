package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	for _, addr := range addrs {
		n, ip := ip_scan(addr.String())
		fmt.Printf("%v %v ==> %d %v\n", addr.Network(), addr.String(), n, ip)
	}
}
