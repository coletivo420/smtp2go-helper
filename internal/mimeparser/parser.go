// SPDX-License-Identifier: GPL-3.0-or-later
package mimeparser

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"sort"
	"strings"

	"github.com/coletivo420/smtp2go-helper/internal/message"
	"golang.org/x/net/html/charset"
)

var allowedHeaders = map[string]bool{"reply-to": true, "message-id": true, "in-reply-to": true, "references": true, "auto-submitted": true, "precedence": true, "list-id": true, "list-unsubscribe": true, "list-unsubscribe-post": true, "date": true}

const (
	MaxMIMEBytes        = 10_240_000
	MaxHeaderBytes      = 64 * 1024
	MaxHeaderCount      = 200
	MaxHeaderLine       = 16 * 1024
	MaxSubjectBytes     = 8192
	MaxMIMEParts        = 256
	MaxMIMEDepth        = 20
	MaxAttachmentCount  = 32
	MaxAttachmentBytes  = 8 * 1024 * 1024
	MaxTotalBinaryBytes = 9 * 1024 * 1024
	MaxFilenameBytes    = 255
	MaxContentIDBytes   = 512
	MaxParsedPartBytes  = 20 * 1024 * 1024
)

type parseState struct {
	parts, attachments, inlines int
	visitedBytes, binaryBytes   int
}

func Parse(raw []byte, recipient, envelopeSender, defaultSender string) (message.Message, error) {
	var out message.Message
	if len(raw) > MaxMIMEBytes {
		return out, errors.New("MIME message exceeds size limit")
	}
	if err := validateHeaderBlock(raw); err != nil {
		return out, err
	}
	out.To = recipient
	r, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return out, fmt.Errorf("parse RFC message: %w", err)
	}
	out.Subject = decodeHeader(r.Header.Get("Subject"))
	if len(out.Subject) > MaxSubjectBytes || strings.ContainsAny(out.Subject, "\r\n\x00") {
		return out, errors.New("Subject header exceeds safe limit or contains controls")
	}
	if from := r.Header.Get("From"); from != "" {
		a, e := mail.ParseAddress(decodeHeader(from))
		if e != nil {
			return out, errors.New("MIME From is invalid")
		}
		out.From = a.String()
	} else if envelopeSender != "" {
		out.From = envelopeSender
	} else if defaultSender != "" {
		a, e := mail.ParseAddress(defaultSender)
		if e != nil {
			return out, errors.New("default sender invalid")
		}
		out.From = a.Address
	} else {
		return out, errors.New("no valid From, envelope sender, or default sender")
	}
	headerNames := make([]string, 0, len(r.Header))
	for k := range r.Header {
		headerNames = append(headerNames, k)
	}
	sort.Strings(headerNames)
	for _, k := range headerNames {
		vals := r.Header[k]
		name := strings.ToLower(k)
		if !allowedHeaders[name] && !strings.HasPrefix(name, "x-") {
			continue
		}
		if strings.ContainsAny(k, "\r\n\x00") {
			continue
		}
		for _, v := range vals {
			v = decodeHeader(v)
			if len(v) > MaxSubjectBytes || strings.ContainsAny(v, "\r\n\x00") {
				return out, errors.New("custom header exceeds safe limit or contains controls")
			}
			if len(out.CustomHeaders) >= 128 {
				return out, errors.New("too many custom headers")
			}
			out.CustomHeaders = append(out.CustomHeaders, message.Header{Name: k, Value: v})
		}
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return out, errors.New("read MIME body")
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain; charset=us-ascii"
	}
	state := &parseState{}
	if err = parseEntity(textproto.MIMEHeader(r.Header), data, &out, ct, false, 0, state); err != nil {
		return out, err
	}
	return out, nil
}

func validateHeaderBlock(raw []byte) error {
	sep := bytes.Index(raw, []byte("\r\n\r\n"))
	if sep < 0 {
		sep = bytes.Index(raw, []byte("\n\n"))
	}
	if sep < 0 {
		return errors.New("MIME header/body separator is missing")
	}
	if sep > MaxHeaderBytes {
		return errors.New("MIME header block exceeds 64 KiB")
	}
	block := raw[:sep]
	count := 0
	for _, line := range bytes.Split(block, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) > MaxHeaderLine {
			return errors.New("MIME header line exceeds 16 KiB")
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			count++
		}
	}
	if count > MaxHeaderCount {
		return errors.New("too many MIME headers")
	}
	return nil
}

func decodeHeader(s string) string {
	d := new(mime.WordDecoder)
	d.CharsetReader = charset.NewReaderLabel
	v, e := d.DecodeHeader(s)
	if e != nil {
		return s
	}
	return v
}

func parseEntity(h textproto.MIMEHeader, body []byte, out *message.Message, contentType string, related bool, depth int, state *parseState) error {
	if depth > MaxMIMEDepth {
		return errors.New("MIME nesting too deep")
	}
	if err := validatePartHeaders(h); err != nil {
		return err
	}
	media, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return errors.New("invalid MIME content type")
	}
	media = strings.ToLower(media)
	if strings.HasPrefix(media, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return errors.New("multipart boundary missing")
		}
		mr := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			p, e := mr.NextRawPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				return errors.New("invalid multipart body")
			}
			state.parts++
			if state.parts > MaxMIMEParts {
				return errors.New("too many MIME parts")
			}
			pb, e := io.ReadAll(io.LimitReader(p, MaxMIMEBytes+1))
			if e != nil || len(pb) > MaxMIMEBytes {
				return errors.New("MIME part exceeds size limit")
			}
			state.visitedBytes += len(pb)
			if state.visitedBytes > MaxParsedPartBytes {
				return errors.New("MIME parsing work exceeds 20 MiB limit")
			}
			ph := textproto.MIMEHeader(p.Header)
			if e = parseEntity(ph, pb, out, ph.Get("Content-Type"), media == "multipart/related" || related, depth+1, state); e != nil {
				return e
			}
		}
		return nil
	}
	decoded, err := decodeTransfer(body, h.Get("Content-Transfer-Encoding"))
	if err != nil {
		return errors.New("invalid MIME transfer encoding")
	}
	disp, dp, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	filename := dp["filename"]
	if filename == "" {
		_, pp, _ := mime.ParseMediaType(contentType)
		filename = pp["name"]
	}
	filename = decodeHeader(filename)
	cid := strings.Trim(strings.TrimSpace(h.Get("Content-ID")), "<>")
	if len(filename) > MaxFilenameBytes || strings.ContainsAny(filename, "\r\n\x00/\\") {
		return errors.New("unsafe or oversized attachment filename")
	}
	if len(cid) > MaxContentIDBytes || strings.ContainsAny(cid, "\r\n\x00") {
		return errors.New("unsafe or oversized Content-ID")
	}
	if (disp == "attachment" || filename != "" || ((disp == "inline" || cid != "") && !strings.HasPrefix(media, "text/"))) && !(media == "text/plain" && filename == "") {
		mt := media
		if mt == "" {
			mt = "application/octet-stream"
		}
		if len(decoded) > MaxAttachmentBytes {
			return errors.New("attachment exceeds 8 MiB limit")
		}
		state.binaryBytes += len(decoded)
		if state.binaryBytes > MaxTotalBinaryBytes {
			return errors.New("attachments exceed 9 MiB total limit")
		}
		if disp == "inline" || cid != "" || related {
			state.inlines++
			if state.inlines > MaxAttachmentCount {
				return errors.New("too many inline attachments")
			}
			out.Inlines = append(out.Inlines, message.Inline{ContentID: cid, Filename: filename, MIMEType: mt, Bytes: decoded})
		} else {
			state.attachments++
			if state.attachments > MaxAttachmentCount {
				return errors.New("too many attachments")
			}
			out.Attachments = append(out.Attachments, message.Attachment{Filename: filename, MIMEType: mt, Bytes: decoded})
		}
		return nil
	}
	if media == "text/plain" || media == "text/html" {
		text, e := toUTF8(decoded, params["charset"])
		if e != nil {
			return errors.New("unsupported or invalid text charset")
		}
		if media == "text/plain" {
			out.TextBody = text
		} else {
			out.HTMLBody = text
		}
	}
	return nil
}

func validatePartHeaders(h textproto.MIMEHeader) error {
	count, total := 0, 0
	for name, values := range h {
		count++
		if len(name) > 255 {
			return errors.New("MIME header name exceeds safe limit")
		}
		for _, value := range values {
			if strings.ContainsAny(value, "\r\n\x00") || len(value) > MaxHeaderLine {
				return errors.New("MIME part header exceeds safe limit")
			}
			total += len(name) + len(value)
		}
	}
	if count > MaxHeaderCount || total > MaxHeaderBytes {
		return errors.New("MIME part has too many or oversized headers")
	}
	return nil
}

func decodeTransfer(b []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "7bit", "8bit", "binary":
		return b, nil
	case "base64":
		return io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(b)))
	case "quoted-printable":
		if !validQuotedPrintable(b) {
			return nil, errors.New("malformed quoted-printable escape")
		}
		return io.ReadAll(quotedprintable.NewReader(bytes.NewReader(b)))
	default:
		return nil, errors.New("unsupported transfer encoding")
	}
}

func validQuotedPrintable(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] != '=' {
			continue
		}
		if i+2 < len(b) && isHex(b[i+1]) && isHex(b[i+2]) {
			i += 2
			continue
		}
		if i+1 < len(b) && b[i+1] == '\n' {
			i++
			continue
		}
		if i+2 < len(b) && b[i+1] == '\r' && b[i+2] == '\n' {
			i += 2
			continue
		}
		return false
	}
	return true
}

func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
func toUTF8(b []byte, label string) (string, error) {
	if label == "" || strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "us-ascii") {
		return string(b), nil
	}
	r, e := charset.NewReaderLabel(label, bytes.NewReader(b))
	if e != nil {
		return "", e
	}
	out, e := io.ReadAll(r)
	if e != nil {
		return "", e
	}
	return string(out), nil
}
