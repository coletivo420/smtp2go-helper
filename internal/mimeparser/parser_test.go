// SPDX-License-Identifier: GPL-3.0-or-later
package mimeparser

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func parseTest(t *testing.T, body string) (string, string, string, string) {
	t.Helper()
	m, err := Parse([]byte(body), "b@example.com", "", "fallback@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return m.From, m.To, m.Subject, m.TextBody + "\x00" + m.HTMLBody
}

func TestPlainUTF8AndEnvelopeOnly(t *testing.T) {
	raw := "From: Sender <sender@example.com>\r\nTo: a@example.com, b@example.com\r\nCc: c@example.com\r\nBcc: secret@example.com\r\nSubject: =?UTF-8?Q?Ol=C3=A1?=\r\nMessage-ID: <id@example.com>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello"
	f, to, sub, body := parseTest(t, raw)
	if f != `"Sender" <sender@example.com>` || to != "b@example.com" || sub != "Olá" || body != "hello\x00" {
		t.Fatalf("parsed values: %q %q %q %q", f, to, sub, body)
	}
}

func TestMultipartAlternativeAndCharsets(t *testing.T) {
	raw := "From: x@example.com\r\nSubject: alt\r\nContent-Type: multipart/alternative; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nOl=E1\r\n--x\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<b>Olá</b>\r\n--x--\r\n"
	m, err := Parse([]byte(raw), "to@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.TextBody != "Olá" || m.HTMLBody != "<b>Olá</b>" {
		t.Fatalf("text/html mismatch: %q / %q", m.TextBody, m.HTMLBody)
	}
}

func TestBase64AndAttachment(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte("pdf bytes"))
	raw := "From: x@example.com\r\nSubject: file\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nbody\r\n--x\r\nContent-Type: application/pdf; name*=utf-8''relat%C3%B3rio.pdf\r\nContent-Disposition: attachment; filename*=utf-8''relat%C3%B3rio.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\n" + blob + "\r\n--x--\r\n"
	m, err := Parse([]byte(raw), "to@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Filename != "relatório.pdf" || string(m.Attachments[0].Bytes) != "pdf bytes" || m.Attachments[0].MIMEType != "application/pdf" {
		t.Fatalf("attachment not mapped: %#v", m.Attachments)
	}
}

func TestInlineCIDAndHeadersAllowlist(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte("png"))
	raw := "From: x@example.com\r\nSubject: inline\r\nDKIM-Signature: secret\r\nX-Test: ok\r\nTo: hidden@example.com\r\nContent-Type: multipart/related; boundary=x\r\n\r\n--x\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<img src=\"cid:pic\">\r\n--x\r\nContent-Type: image/png\r\nContent-ID: <pic>\r\nContent-Disposition: inline; filename=pic.png\r\nContent-Transfer-Encoding: base64\r\n\r\n" + blob + "\r\n--x--\r\n"
	m, err := Parse([]byte(raw), "only@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Inlines) != 1 || m.Inlines[0].ContentID != "pic" || string(m.Inlines[0].Bytes) != "png" || m.Inlines[0].MIMEType != "image/png" {
		t.Fatalf("inline mapping: %#v", m.Inlines)
	}
	for _, h := range m.CustomHeaders {
		if strings.EqualFold(h.Name, "DKIM-Signature") || strings.EqualFold(h.Name, "To") {
			t.Fatalf("unsafe header forwarded: %s", h.Name)
		}
	}
}

func TestEmptyEnvelopeSenderFallsBackToDefault(t *testing.T) {
	m, err := Parse([]byte("Subject: no from\r\nContent-Type: text/plain\r\n\r\nbody"), "to@example.com", "", "default@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if m.From != "default@example.com" {
		t.Fatalf("sender=%q", m.From)
	}
}

func TestHeadersAndFoldedSubject(t *testing.T) {
	raw := "From: a@example.com\r\nTo: private@example.com\r\nSubject: =?UTF-8?Q?Ol=C3=A1?=\r\nReply-To: help@example.com\r\nMessage-ID: <m@example.com>\r\nReferences: <a@example.com>\r\nList-Unsubscribe: <https://example.com/u>\r\nDKIM-Signature: v=1;\r\n folded\r\nContent-Type: text/plain\r\n\r\nbody"
	m, err := Parse([]byte(raw), "envelope@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject != "Olá" || m.To != "envelope@example.com" {
		t.Fatalf("headers: %+v", m)
	}
	want := map[string]string{"Reply-To": "help@example.com", "Message-Id": "<m@example.com>", "References": "<a@example.com>", "List-Unsubscribe": "<https://example.com/u>"}
	for _, h := range m.CustomHeaders {
		if v, ok := want[h.Name]; ok {
			if h.Value != v {
				t.Errorf("%s=%q", h.Name, h.Value)
			}
			delete(want, h.Name)
		}
		if strings.EqualFold(h.Name, "DKIM-Signature") || strings.EqualFold(h.Name, "To") {
			t.Errorf("forbidden header %s", h.Name)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing headers: %v", want)
	}
}
func TestMissingSenderAndMalformedMIME(t *testing.T) {
	if _, err := Parse([]byte("Subject: x\r\n\r\nb"), "to@example.com", "", ""); err == nil {
		t.Fatal("missing sender accepted")
	}
	if _, err := Parse([]byte("not a header\r\n\r\nb"), "to@example.com", "", ""); err == nil {
		t.Fatal("bad MIME accepted")
	}
}

func TestHeaderSeparatorUsesEarliestSupportedDelimiter(t *testing.T) {
	t.Run("LF headers with CRLF blank line inside body", func(t *testing.T) {
		raw := []byte("From: sender@example.com\nSubject: LF message\nContent-Type: text/plain; charset=utf-8\n\nfirst line\r\n\r\nbody delimiter belongs to body")
		m, err := Parse(raw, "recipient@example.com", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if m.Subject != "LF message" || m.TextBody != "first line\r\n\r\nbody delimiter belongs to body" {
			t.Fatalf("wrong separator selected: subject=%q body=%q", m.Subject, m.TextBody)
		}
	})

	t.Run("CRLF headers with LF blank line inside body", func(t *testing.T) {
		raw := []byte("From: sender@example.com\r\nSubject: CRLF message\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nfirst line\n\nbody delimiter belongs to body")
		m, err := Parse(raw, "recipient@example.com", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if m.Subject != "CRLF message" || m.TextBody != "first line\n\nbody delimiter belongs to body" {
			t.Fatalf("wrong separator selected: subject=%q body=%q", m.Subject, m.TextBody)
		}
	})

	t.Run("LF only", func(t *testing.T) {
		m, err := Parse([]byte("From: sender@example.com\nSubject: LF only\n\nbody"), "recipient@example.com", "", "")
		if err != nil || m.TextBody != "body" {
			t.Fatalf("LF-only MIME parse failed: body=%q error=%v", m.TextBody, err)
		}
	})
	t.Run("missing delimiter", func(t *testing.T) {
		if _, err := Parse([]byte("From: sender@example.com\nSubject: no separator"), "recipient@example.com", "", ""); err == nil {
			t.Fatal("message without header/body separator accepted")
		}
	})
}

func TestNestedMultipartAndCommonAttachmentTypes(t *testing.T) {
	// Exercise mixed -> alternative nesting and multiple common binary formats.
	parts := []struct {
		name, media, data string
	}{
		{"documento.pdf", "application/pdf", "%PDF-test"},
		{"imagem.png", "image/png", "PNG-test"},
		{"foto.jpeg", "image/jpeg", "JPEG-test"},
		{"backup.zip", "application/zip", "ZIP-test"},
		{"relat%C3%B3rio.txt", "text/plain", "texto"},
	}
	var raw strings.Builder
	raw.WriteString("From: sender@example.com\r\nSubject: nested\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=outer\r\n\r\n")
	raw.WriteString("--outer\r\nContent-Type: multipart/alternative; boundary=alt\r\n\r\n")
	raw.WriteString("--alt\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nplain\r\n--alt\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<b>html</b>\r\n--alt--\r\n")
	for _, p := range parts {
		encoded := base64.StdEncoding.EncodeToString([]byte(p.data))
		raw.WriteString("--outer\r\nContent-Type: " + p.media + "\r\nContent-Disposition: attachment; filename*=utf-8''" + p.name + "\r\nContent-Transfer-Encoding: base64\r\n\r\n" + encoded + "\r\n")
	}
	raw.WriteString("--outer--\r\n")

	m, err := Parse([]byte(raw.String()), "only@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.TextBody != "plain" || m.HTMLBody != "<b>html</b>" {
		t.Fatalf("nested alternatives not mapped: text=%q html=%q", m.TextBody, m.HTMLBody)
	}
	if len(m.Attachments) != len(parts) {
		t.Fatalf("got %d attachments, want %d", len(m.Attachments), len(parts))
	}
	for i, p := range parts {
		got := m.Attachments[i]
		wantName := strings.ReplaceAll(p.name, "%C3%B3", "ó")
		if got.Filename != wantName || got.MIMEType != p.media || string(got.Bytes) != p.data {
			t.Errorf("attachment %d mismatch: %#v", i, got)
		}
	}
}

func TestMIMEStructuralLimitsAndMalformedEncodings(t *testing.T) {
	base := func(contentType, body string) []byte {
		return []byte("From: sender@example.com\r\nSubject: limited\r\nContent-Type: " + contentType + "\r\n\r\n" + body)
	}
	tests := []struct {
		name string
		raw  []byte
	}{
		{"header-line", []byte("From: sender@example.com\r\nSubject: " + strings.Repeat("x", MaxHeaderLine+1) + "\r\n\r\nbody")},
		{"oversized-message", append([]byte("From: sender@example.com\r\n\r\n"), bytesOf(MaxMIMEBytes+1)...)},
		{"malformed-base64", base("application/octet-stream\r\nContent-Transfer-Encoding: base64", "%%%bad")},
		{"malformed-quoted-printable", base("text/plain\r\nContent-Transfer-Encoding: quoted-printable", "bad=GZ")},
		{"malformed-content-type", base("multipart/mixed; boundary=\"unterminated", "body")},
		{"unknown-charset", base("text/plain; charset=x-unknown-codex", "body")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.raw, "to@example.com", "", "fallback@example.com"); err == nil {
				t.Fatal("hostile or malformed MIME accepted")
			}
		})
	}
}

func TestMIMEPartDepthCountAttachmentAndMetadataLimits(t *testing.T) {
	deep := "Content-Type: text/plain\r\n\r\nleaf"
	for i := 0; i < MaxMIMEDepth+2; i++ {
		boundary := fmt.Sprintf("b%d", i)
		deep = fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n--%s\r\n%s\r\n--%s--\r\n", boundary, boundary, strings.ReplaceAll(deep, "\r\n", "\r\n"), boundary)
	}
	if _, err := Parse([]byte("From: a@example.com\r\nSubject: x\r\n"+deep), "to@example.com", "", ""); err == nil {
		t.Fatal("excessive multipart depth accepted")
	}

	var many strings.Builder
	many.WriteString("Content-Type: multipart/mixed; boundary=m\r\n\r\n")
	for i := 0; i < MaxMIMEParts+1; i++ {
		many.WriteString("--m\r\nContent-Type: text/plain\r\n\r\nx\r\n")
	}
	many.WriteString("--m--\r\n")
	if _, err := Parse([]byte("From: a@example.com\r\nSubject: x\r\n"+many.String()), "to@example.com", "", ""); err == nil {
		t.Fatal("excessive multipart count accepted")
	}

	var attachments strings.Builder
	attachments.WriteString("Content-Type: multipart/mixed; boundary=a\r\n\r\n")
	for i := 0; i < MaxAttachmentCount+1; i++ {
		attachments.WriteString(fmt.Sprintf("--a\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=f%d.bin\r\n\r\nx\r\n", i))
	}
	attachments.WriteString("--a--\r\n")
	if _, err := Parse([]byte("From: a@example.com\r\nSubject: x\r\n"+attachments.String()), "to@example.com", "", ""); err == nil {
		t.Fatal("excessive attachment count accepted")
	}

	longName := "Content-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=" + strings.Repeat("f", MaxFilenameBytes+1) + "\r\n\r\nx"
	if _, err := Parse([]byte("From: a@example.com\r\nSubject: x\r\n"+longName), "to@example.com", "", ""); err == nil {
		t.Fatal("oversized attachment filename accepted")
	}
	longCID := "Content-Type: image/png\r\nContent-ID: <" + strings.Repeat("a", MaxContentIDBytes+1) + ">\r\n\r\nx"
	if _, err := Parse([]byte("From: a@example.com\r\nSubject: x\r\n"+longCID), "to@example.com", "", ""); err == nil {
		t.Fatal("oversized Content-ID accepted")
	}
}

func bytesOf(n int) []byte { return []byte(strings.Repeat("x", n)) }

func FuzzParseMIMENoPanic(f *testing.F) {
	f.Add([]byte("From: sender@example.com\r\nSubject: x\r\nContent-Type: text/plain\r\n\r\nhello"))
	f.Add([]byte("Content-Type: multipart/mixed; boundary=x\r\n\r\n--x--\r\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxMIMEBytes+1 {
			t.Skip()
		}
		_, _ = Parse(raw, "to@example.com", "", "fallback@example.com")
	})
}
