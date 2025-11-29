//go:build !unix

package main

import (
	"log"
	"runtime"
)

func dropPrivilegesToUser(username string) error {
	log.Printf("dropPrivilegesToUser: missed dropping privileges on %s", runtime.GOOS)
	return nil
}
