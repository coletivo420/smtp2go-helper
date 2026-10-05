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
