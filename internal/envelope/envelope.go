// SPDX-License-Identifier: GPL-3.0-or-later
package envelope

import (
	"errors"
	"net/mail"
	"strings"
)

func ParseRecipient(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return "", errors.New("empty or unsafe recipient")
	}
	a, err := mail.ParseAddress(raw)
	if err != nil {
		return "", errors.New("recipient syntax invalid")
	}
	if a.Address == "" || strings.ContainsAny(a.Address, "\r\n\x00") {
		return "", errors.New("recipient syntax invalid")
	}
	return a.Address, nil
}

func ParseOptionalSender(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return "", errors.New("unsafe sender")
	}
	a, err := mail.ParseAddress(raw)
	if err != nil {
		return "", errors.New("sender syntax invalid")
	}
	return a.Address, nil
}
