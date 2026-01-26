//go:build unix

package main

import (
	"fmt"
	"log/slog"

	"golang.org/x/sys/unix"
)

func chroot(path string) error {
	if path == "" {
		return nil
	}

	if err := unix.Chroot(path); err != nil {
		return fmt.Errorf("chroot %s: %w", path, err)
	}
	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}

	slog.Debug("chroot", "path", path)
	return nil
}

func dropPrivileges(uid, gid int) error {
	if uid == 0 && gid == 0 {
		return nil
	}

	if gid != 0 {
		if err := unix.Setgid(gid); err != nil {
			return fmt.Errorf("setgid %d: %w", gid, err)
		}
	}
	if uid != 0 {
		if err := unix.Setuid(uid); err != nil {
			return fmt.Errorf("setuid %d: %w", uid, err)
		}
	}

	slog.Debug("dropped privileges", "uid", uid, "gid", gid)
	return nil
}

func sandbox(chrootPath string, uid, gid int) error {
	// Order: chroot, drop privileges, pledge
	if err := chroot(chrootPath); err != nil {
		return err
	}
	if err := dropPrivileges(uid, gid); err != nil {
		return err
	}
	return pledge()
}
