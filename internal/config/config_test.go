// SPDX-License-Identifier: GPL-3.0-or-later
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestUnknownFieldRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"endpoint":"https://api.smtp2go.com/v3/email/send","timeout_seconds":30,"fastaccept":false,"default_sender":"","max_message_bytes":10485760,"log_level":"info","unexpected":true}`), 0600)
	if _, err := Load(p); err == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestEndpointAndLimitStrict(t *testing.T) {
	c := Defaults()
	c.Endpoint = "https://api.smtp2go.com/v3/email/mime"
	if c.Validate() == nil {
		t.Fatal("wrong endpoint accepted")
	}
	c = Defaults()
	c.MaxMessageBytes = 10485761
	if c.Validate() == nil {
		t.Fatal("oversize setting accepted")
	}
}

func TestLoadRejectsSymlinkNonRegularAndOversizedConfig(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(valid, []byte(`{"endpoint":"https://api.smtp2go.com/v3/email/send","timeout_seconds":30,"fastaccept":false,"default_sender":"","max_message_bytes":10485760,"log_level":"info"}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.json")
	if err := os.Symlink(valid, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("config symlink accepted")
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("directory accepted as config")
	}
	large := filepath.Join(dir, "large.json")
	if err := os.WriteFile(large, make([]byte, maxConfigBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(large); err == nil {
		t.Fatal("oversized config accepted")
	}
}

func TestLoadRejectsWorldReadableConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"endpoint":"https://api.smtp2go.com/v3/email/send","timeout_seconds":30,"fastaccept":false,"default_sender":"","max_message_bytes":10485760,"log_level":"info"}`)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("world-readable config accepted")
	}
}
