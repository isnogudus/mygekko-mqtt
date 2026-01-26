//go:build !unix

package main

func chroot(path string) error {
	return nil
}

func dropPrivileges(uid, gid int) error {
	return nil
}

func sandbox(chrootPath string, uid, gid int) error {
	return nil
}
