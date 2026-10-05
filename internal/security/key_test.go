// SPDX-License-Identifier: GPL-3.0-or-later
package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeKey = "api-01234567890123456789012345678901"

func makeKeyFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "api.key")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadAPIKeySecureFile(t *testing.T) {
	p := makeKeyFile(t, fakeKey+"\n", 0640)
	got, err := readAPIKey(p, uint32(os.Getuid()), uint32(os.Getgid()))
	if err != nil || got != fakeKey {
		t.Fatalf("key read failed: value length=%d error=%v", len(got), err)
	}
}

func TestReadAPIKeyRejectsUnsafeFiles(t *testing.T) {
	tests := []struct {
		name string
		data string
		mode os.FileMode
		uid  uint32
	}{
		{"empty", "", 0640, uint32(os.Getuid())},
		{"NUL", fakeKey + "\x00", 0640, uint32(os.Getuid())},
		{"invalid-format", "not-a-key", 0640, uint32(os.Getuid())},
		{"world-readable", fakeKey, 0644, uint32(os.Getuid())},
		{"unexpected-owner", fakeKey, 0640, uint32(os.Getuid()) + 1},
		{"oversized", fakeKey + strings.Repeat("x", 300), 0640, uint32(os.Getuid())},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := makeKeyFile(t, tc.data, tc.mode)
			if got, err := readAPIKey(p, tc.uid, uint32(os.Getgid())); err == nil || got != "" {
				t.Fatalf("unsafe key accepted (length=%d)", len(got))
			}
		})
	}
}

func TestReadAPIKeyRejectsUnexpectedGroup(t *testing.T) {
	p := makeKeyFile(t, fakeKey, 0640)
	if got, err := readAPIKey(p, uint32(os.Getuid()), uint32(os.Getgid())+1); err == nil || got != "" {
		t.Fatalf("unexpected group accepted (length=%d)", len(got))
	}
}

func TestReadAPIKeyRejectsSymlinkAndNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	target := makeKeyFile(t, fakeKey, 0640)
	link := filepath.Join(dir, "api.key")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got, err := readAPIKey(link, uint32(os.Getuid()), uint32(os.Getgid())); err == nil || got != "" {
		t.Fatalf("symlink accepted (length=%d)", len(got))
	}
	if got, err := readAPIKey(dir, uint32(os.Getuid()), uint32(os.Getgid())); err == nil || got != "" {
		t.Fatalf("directory accepted as key (length=%d)", len(got))
	}
}
