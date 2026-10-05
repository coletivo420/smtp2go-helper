// SPDX-License-Identifier: GPL-3.0-or-later
package security

import (
	"errors"
	"io"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const DefaultKeyPath = "/etc/smtp2go-helper/api.key"
const DefaultConfigPath = "/etc/smtp2go-helper/config.json"

var apiKeyPattern = regexp.MustCompile(`^api-[A-Za-z0-9]{32}$`)

func ReadAPIKey(path string) (string, error) {
	gid, err := serviceGID()
	if err != nil {
		return "", err
	}
	return readAPIKey(path, 0, uint32(gid))
}

func CheckProtectedConfig() error {
	gid, err := serviceGID()
	if err != nil {
		return err
	}
	dir, err := os.Lstat("/etc/smtp2go-helper")
	if err != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 || dir.Mode().Perm() != 0750 {
		return errors.New("config directory must be root:smtp2go-helper mode 0750")
	}
	meta, ok := dir.Sys().(*syscall.Stat_t)
	if !ok || meta.Uid != 0 || meta.Gid != uint32(gid) {
		return errors.New("config directory must be root:smtp2go-helper mode 0750")
	}
	fd, err := syscall.Open(DefaultConfigPath, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return errors.New("config file unavailable or symlink")
	}
	defer syscall.Close(fd)
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Uid != 0 || st.Gid != uint32(gid) || st.Mode&07777 != 0640 || st.Size > 64*1024 {
		return errors.New("config file must be root:smtp2go-helper mode 0640 and at most 64 KiB")
	}
	return nil
}

func serviceGID() (uint64, error) {
	grp, err := user.LookupGroup("smtp2go-helper")
	if err != nil {
		return 0, errors.New("key group unavailable")
	}
	gid, err := strconv.ParseUint(grp.Gid, 10, 32)
	if err != nil {
		return 0, errors.New("key group unavailable")
	}
	return gid, nil
}

// readAPIKey opens without following symlinks and validates the opened inode,
// avoiding a stat/open race. The expected IDs are parameters so tests can use
// their unprivileged temporary-file owner without weakening production checks.
func readAPIKey(path string, expectedUID, expectedGID uint32) (string, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", errors.New("key file unavailable or symlink")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return "", errors.New("key file metadata unavailable")
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Mode&07777 != 0640 || stat.Uid != expectedUID || stat.Gid != expectedGID {
		return "", errors.New("key file owner must be root:smtp2go-helper")
	}
	b, err := io.ReadAll(io.LimitReader(f, 257))
	if err != nil {
		return "", errors.New("key file unreadable")
	}
	k := strings.TrimRight(string(b), "\r\n")
	if len(b) > 256 || strings.ContainsRune(k, '\x00') || !apiKeyPattern.MatchString(k) {
		return "", errors.New("key format invalid")
	}
	return k, nil
}
