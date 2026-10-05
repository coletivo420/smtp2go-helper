// SPDX-License-Identifier: GPL-3.0-or-later
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"syscall"
	"time"
)

const DefaultPath = "/etc/smtp2go-helper/config.json"
const maxConfigBytes = 64 * 1024

type Config struct {
	Endpoint        string `json:"endpoint"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	FastAccept      bool   `json:"fastaccept"`
	DefaultSender   string `json:"default_sender"`
	MaxMessageBytes int    `json:"max_message_bytes"`
	LogLevel        string `json:"log_level"`
}

func Defaults() Config {
	return Config{Endpoint: "https://api.smtp2go.com/v3/email/send", TimeoutSeconds: 30, MaxMessageBytes: 10240000, LogLevel: "info"}
}
func (c Config) Timeout() time.Duration { return time.Duration(c.TimeoutSeconds) * time.Second }

func Load(path string) (Config, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return Config{}, errors.New("config file unavailable or symlink")
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Mode&0137 != 0 || st.Size < 0 || st.Size > maxConfigBytes {
		return Config{}, errors.New("config file must be a regular, restricted file no larger than 64 KiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil || len(b) > maxConfigBytes {
		return Config{}, errors.New("config file read failed or exceeded 64 KiB")
	}
	var c Config
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err = d.Decode(&struct{}{}); err != io.EOF {
		return Config{}, errors.New("config must contain one JSON object")
	}
	if err = c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host != "api.smtp2go.com" || u.Path != "/v3/email/send" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must be https://api.smtp2go.com/v3/email/send")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 300 {
		return errors.New("timeout_seconds must be between 1 and 300")
	}
	if c.MaxMessageBytes < 1024 || c.MaxMessageBytes > 10240000 {
		return errors.New("max_message_bytes must be between 1024 and 10240000")
	}
	if c.DefaultSender != "" {
		if _, err := mail.ParseAddress(c.DefaultSender); err != nil {
			return errors.New("default_sender must be a valid RFC 5322 address")
		}
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("log_level must be debug, info, warn, or error")
	}
	return nil
}
