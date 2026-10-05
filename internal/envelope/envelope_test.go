// SPDX-License-Identifier: GPL-3.0-or-later
package envelope

import "testing"

func TestEnvelope(t *testing.T) {
	if _, e := ParseRecipient("a@example.com\nBcc:x@example.com"); e == nil {
		t.Fatal("newline accepted")
	}
	if _, e := ParseRecipient("bad address"); e == nil {
		t.Fatal("invalid recipient accepted")
	}
	if _, e := ParseRecipient("a@example.com,b@example.com"); e == nil {
		t.Fatal("multiple envelope recipients accepted")
	}
	if s, e := ParseOptionalSender(""); e != nil || s != "" {
		t.Fatalf("empty return-path rejected: %q %v", s, e)
	}
	if s, e := ParseOptionalSender("sender@example.com"); e != nil || s != "sender@example.com" {
		t.Fatalf("sender: %q %v", s, e)
	}
}

func TestRecipientRejectsNULAndControls(t *testing.T) {
	for _, raw := range []string{"a@example.com\x00", "a@example.com\rBcc:x@example.com", "\n"} {
		if _, err := ParseRecipient(raw); err == nil {
			t.Fatalf("unsafe recipient accepted: %q", raw)
		}
	}
}

func FuzzParseRecipientNoPanic(f *testing.F) {
	f.Add("user@example.com")
	f.Add("a@example.com\r\nBcc:x@example.com")
	f.Add("\x00")
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = ParseRecipient(value)
	})
}
