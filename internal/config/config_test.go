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
