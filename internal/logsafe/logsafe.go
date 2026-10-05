// SPDX-License-Identifier: GPL-3.0-or-later
package logsafe

import (
	"regexp"
	"strings"
)

var controls = regexp.MustCompile(`[\x00-\x1f\x7f]`)
var key = regexp.MustCompile(`(?i)api-[a-z0-9]{32}`)
var email = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
var longToken = regexp.MustCompile(`[A-Za-z0-9+/]{80,}={0,2}`)
var sensitiveJSON = regexp.MustCompile(`(?i)("(?:api[_-]?key|mime_email|payload|fileblob|attachments?|html_body|text_body|body)"\s*:\s*)("(?:\\.|[^"])*"|[^,}\s]+)`)
var sensitiveHeader = regexp.MustCompile(`(?i)authorization\s*[:=]\s*(?:bearer\s+)?[^\s,;]+|(?:x-smtp2go-api-key|api[_-]?key|token|secret)\s*[:= ]\s*[^\s,;]+`)
var jwt = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)

func Text(s string) string {
	s = controls.ReplaceAllString(s, " ")
	s = key.ReplaceAllString(s, "[redacted]")
	s = jwt.ReplaceAllString(s, "[token redacted]")
	s = sensitiveJSON.ReplaceAllString(s, `${1}"[redacted]"`)
	s = sensitiveHeader.ReplaceAllString(s, "[credential redacted]")
	s = longToken.ReplaceAllString(s, "[data redacted]")
	s = email.ReplaceAllStringFunc(s, func(v string) string {
		at := strings.LastIndex(v, "@")
		if at < 0 {
			return "[address]"
		}
		return "[address]@" + v[at+1:]
	})
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
func Token(s string) string {
	s = Text(s)
	if len(s) > 128 {
		return s[:128]
	}
	return s
}
