// SPDX-License-Identifier: GPL-3.0-or-later
package mimeparser

import (
	"encoding/base64"
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
