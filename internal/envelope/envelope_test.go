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
