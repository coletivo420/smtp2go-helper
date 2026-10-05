// SPDX-License-Identifier: GPL-3.0-or-later
package message

type Attachment struct {
	Filename string
	MIMEType string
	Bytes    []byte
}

type Inline struct {
	ContentID string
	Filename  string
	MIMEType  string
	Bytes     []byte
}

type Header struct {
	Name  string
	Value string
}

type Message struct {
	From          string
	To            string
	Subject       string
	TextBody      string
	HTMLBody      string
	Attachments   []Attachment
	Inlines       []Inline
	CustomHeaders []Header
}
