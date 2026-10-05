// SPDX-License-Identifier: GPL-3.0-or-later
package mimeparser

import (
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

func Parse(raw []byte, recipient, envelopeSender, defaultSender string) (message.Message, error) {
	var out message.Message
	out.To = recipient
	r, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		return out, fmt.Errorf("parse RFC message: %w", err)
	}
	out.Subject = decodeHeader(r.Header.Get("Subject"))
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
			if strings.ContainsAny(v, "\r\n\x00") {
				continue
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
	if err = parseEntity(textproto.MIMEHeader(r.Header), data, &out, ct, false, 0); err != nil {
		return out, err
	}
	return out, nil
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

func parseEntity(h textproto.MIMEHeader, body []byte, out *message.Message, contentType string, related bool, depth int) error {
	if depth > 30 {
		return errors.New("MIME nesting too deep")
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
		mr := multipart.NewReader(strings.NewReader(string(body)), boundary)
		for {
			p, e := mr.NextRawPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				return errors.New("invalid multipart body")
			}
			pb, e := io.ReadAll(io.LimitReader(p, 10*1024*1024+1))
			if e != nil || len(pb) > 10*1024*1024 {
				return errors.New("MIME part exceeds size limit")
			}
			ph := textproto.MIMEHeader(p.Header)
			if e = parseEntity(ph, pb, out, ph.Get("Content-Type"), media == "multipart/related" || related, depth+1); e != nil {
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
	if (disp == "attachment" || filename != "" || ((disp == "inline" || cid != "") && !strings.HasPrefix(media, "text/"))) && !(media == "text/plain" && filename == "") {
		mt := media
		if mt == "" {
			mt = "application/octet-stream"
		}
		if disp == "inline" || cid != "" || related {
			out.Inlines = append(out.Inlines, message.Inline{ContentID: cid, Filename: filename, MIMEType: mt, Bytes: decoded})
		} else {
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

func decodeTransfer(b []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "7bit", "8bit", "binary":
		return b, nil
	case "base64":
		return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(string(b))))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(b))))
	default:
		return nil, errors.New("unsupported transfer encoding")
	}
}
func toUTF8(b []byte, label string) (string, error) {
	if label == "" || strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "us-ascii") {
		return string(b), nil
	}
	r, e := charset.NewReaderLabel(label, strings.NewReader(string(b)))
	if e != nil {
		return "", e
	}
	out, e := io.ReadAll(r)
	if e != nil {
		return "", e
	}
	return string(out), nil
}
