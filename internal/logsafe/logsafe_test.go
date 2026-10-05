// SPDX-License-Identifier: GPL-3.0-or-later
package logsafe

import (
	"strings"
	"testing"
)

func TestSecretsControlsAndLimit(t *testing.T) {
	s := Text("api-01234567890123456789012345678901\nsecret\t" + strings.Repeat("x", 600))
	if strings.Contains(s, "api-0123") || strings.ContainsAny(s, "\r\n\t") || len(s) > 500 {
		t.Fatalf("unsafe text %q len=%d", s, len(s))
	}
}

func TestMaliciousDiagnosticInputsAreSanitized(t *testing.T) {
	secret := "api-01234567890123456789012345678901"
	tests := []struct {
		name, input, forbidden string
	}{
		{"api-key", "request rejected " + secret, secret},
		{"header", "X-Smtp2go-Api-Key: topsecret", "topsecret"},
		{"authorization", "Authorization: Bearer topsecret", "topsecret"},
		{"json-mime", `{"mime_email":"SGVsbG8=","payload":"private"}`, "SGVsbG8="},
		{"json-body", `{"body":"private body","attachments":["secret"]}`, "private body"},
		{"jwt", "token=eyJabcdefghijk.abcdefghijk.abcdefghijk", "eyJabcdefghijk"},
		{"controls", "bad\r\nheader\x00value", "\r"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Text(tc.input)
			if strings.Contains(got, tc.forbidden) {
				t.Fatalf("unsafe content survived: %q", got)
			}
			if len(got) > 500 || strings.ContainsAny(got, "\r\n\x00") {
				t.Fatalf("unsafe log format: length=%d value=%q", len(got), got)
			}
		})
	}
}

func FuzzTextNoPanic(f *testing.F) {
	f.Add("api-01234567890123456789012345678901")
	f.Add(`{"mime_email":"abc","body":"x"}`)
	f.Fuzz(func(t *testing.T, input string) {
		got := Text(input)
		if len(got) > 500 || strings.ContainsAny(got, "\r\n\x00") {
			t.Fatalf("unsafe sanitizer output")
		}
	})
}
