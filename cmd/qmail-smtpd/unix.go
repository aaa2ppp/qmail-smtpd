//go:build unix

package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func dropPrivilegesToUser(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("can't get uid: %w", err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("can't get gid: %w", err)
	}
	return dropPrivileges(uid, gid)
}

func dropPrivileges(uid, gid int) error {
	if curUID := os.Getuid(); curUID != 0 {
		return fmt.Errorf("only root can drop privileges, cur uid=%d", curUID)
	}
	if err := syscall.Setgid(gid); err != nil {
		return err
	}
	if err := syscall.Setuid(uid); err != nil {
		return err
	}
	if curUID, curGID := syscall.Getuid(), syscall.Getgid(); curUID != uid || curGID != gid {
		return fmt.Errorf("failed to drop privileges, still UID/GID:%d/%d", curUID, curGID)
	}
	return nil
}
