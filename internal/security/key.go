// SPDX-License-Identifier: GPL-3.0-or-later
package security

import (
	"errors"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const DefaultKeyPath = "/etc/smtp2go-helper/api.key"

var apiKeyPattern = regexp.MustCompile(`^api-[A-Za-z0-9]{32}$`)

func ReadAPIKey(path string) (string, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return "", errors.New("key file unavailable")
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0640 {
		return "", errors.New("key file must be regular and mode 0640")
	}
	meta, ok := st.Sys().(*syscall.Stat_t)
	grp, gerr := user.LookupGroup("smtp2go-helper")
	if !ok || gerr != nil || meta.Uid != 0 || strconv.FormatUint(uint64(meta.Gid), 10) != grp.Gid {
		return "", errors.New("key file owner must be root:smtp2go-helper")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("key file unreadable")
	}
	k := strings.TrimRight(string(b), "\r\n")
	if !apiKeyPattern.MatchString(k) {
		return "", errors.New("key format invalid")
	}
	return k, nil
}
