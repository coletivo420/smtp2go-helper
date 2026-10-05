// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/coletivo420/smtp2go-helper/internal/config"
	"github.com/coletivo420/smtp2go-helper/internal/envelope"
	"github.com/coletivo420/smtp2go-helper/internal/logsafe"
	"github.com/coletivo420/smtp2go-helper/internal/mimeparser"
	"github.com/coletivo420/smtp2go-helper/internal/security"
	"github.com/coletivo420/smtp2go-helper/internal/smtp2go"
	"github.com/coletivo420/smtp2go-helper/internal/sysexits"
	"github.com/coletivo420/smtp2go-helper/internal/version"
)

const maxStdinBytes = 10240000

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stderr)) }

func run(args []string, stdin io.Reader, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(os.Stdout, version.String())
		return sysexits.OK
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: smtp2go-helper [--version|doctor|config validate|api permissions] <envelope-sender> <single-recipient>")
		return sysexits.USAGE
	}
	if args[0] == "doctor" {
		return doctor(stderr)
	}
	if len(args) == 2 && args[0] == "config" && args[1] == "validate" {
		_, err := config.Load(config.DefaultPath)
		if err != nil {
			fmt.Fprintf(stderr, "smtp2go-helper: configuration invalid: %s\n", logsafe.Text(err.Error()))
			return sysexits.TEMPFAIL
		}
		fmt.Fprintln(stderr, "smtp2go-helper: configuration valid")
		return sysexits.OK
	}
	if len(args) == 2 && args[0] == "api" && args[1] == "permissions" {
		return permissions(stderr)
	}
	if len(args) != 2 && len(args) != 3 {
		fmt.Fprintln(stderr, "smtp2go-helper: expected envelope sender, exactly one recipient, and optional queue id")
		return sysexits.USAGE
	}

	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: configuration unavailable: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	body, err := io.ReadAll(io.LimitReader(stdin, int64(cfg.MaxMessageBytes)+1))
	if err != nil {
		fmt.Fprintln(stderr, "smtp2go-helper: cannot read message from Postfix")
		return sysexits.TEMPFAIL
	}
	if len(body) == 0 || len(body) > cfg.MaxMessageBytes || len(body) > maxStdinBytes {
		fmt.Fprintln(stderr, "smtp2go-helper: message is empty or exceeds configured limit")
		return sysexits.DATA
	}
	recipient, err := envelope.ParseRecipient(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: invalid envelope recipient: %s\n", logsafe.Text(err.Error()))
		return sysexits.DATA
	}
	sender, _ := envelope.ParseOptionalSender(args[0])
	msg, err := mimeparser.Parse(body, recipient, sender, cfg.DefaultSender)
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: invalid MIME message: %s\n", logsafe.Text(err.Error()))
		return sysexits.DATA
	}
	key, err := security.ReadAPIKey(security.DefaultKeyPath)
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: API key unavailable: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	client := smtp2go.New(cfg.Endpoint, cfg.Timeout(), key, cfg.FastAccept)
	result, err := client.Send(context.Background(), msg)
	if err != nil {
		var apiErr *smtp2go.APIError
		if errors.As(err, &apiErr) {
			fmt.Fprintf(stderr, "smtp2go-helper: %s\n", logsafe.Text(apiErr.Error()))
			return apiErr.ExitCode
		}
		fmt.Fprintf(stderr, "smtp2go-helper: temporary API failure: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	queueID := ""
	if len(args) == 3 {
		queueID = logsafe.Token(args[2])
	}
	fmt.Fprintf(stderr, "smtp2go-helper: accepted queue_id=%s request_id=%s email_id=%s succeeded=%d\n", queueID, logsafe.Token(result.RequestID), logsafe.Token(result.EmailID), result.Succeeded)
	return sysexits.OK
}

func permissions(stderr io.Writer) int {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: configuration unavailable: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	key, err := security.ReadAPIKey(security.DefaultKeyPath)
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: API key unavailable: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	client := smtp2go.New(cfg.Endpoint, cfg.Timeout(), key, cfg.FastAccept)
	perms, err := client.Permissions(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "smtp2go-helper: permission query failed: %s\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	fmt.Fprintf(os.Stdout, "/email/send: %s\n", perms.Send)
	if perms.Send != "allowed" {
		return sysexits.TEMPFAIL
	}
	return sysexits.OK
}

func doctor(stderr io.Writer) int {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		fmt.Fprintf(stderr, "config: ERROR (%s)\n", logsafe.Text(err.Error()))
		return sysexits.TEMPFAIL
	}
	fmt.Fprintf(stderr, "helper: %s\nendpoint: %s\ntimeout: %s\nrecipient limit: ", version.String(), cfg.Endpoint, cfg.Timeout())
	limit, lerr := exec.Command("postconf", "-h", "smtp2go-helper_destination_recipient_limit").Output()
	if lerr != nil {
		fmt.Fprintln(stderr, "unknown (Postfix unavailable)")
		return sysexits.TEMPFAIL
	}
	fmt.Fprintln(stderr, strings.TrimSpace(string(limit)))
	if strings.TrimSpace(string(limit)) != "1" {
		fmt.Fprintln(stderr, "recipient limit must equal 1")
		return sysexits.TEMPFAIL
	}
	transport, e := exec.Command("postconf", "-M", "smtp2go-helper").Output()
	if e != nil || !strings.Contains(string(transport), " pipe ") {
		fmt.Fprintln(stderr, "Postfix transport smtp2go-helper: ERROR")
		return sysexits.TEMPFAIL
	}
	fmt.Fprintln(stderr, "Postfix transport smtp2go-helper: pipe")
	active, activeErr := exec.Command("systemctl", "is-active", "--quiet", "postfix").CombinedOutput()
	if activeErr != nil {
		fmt.Fprintf(stderr, "Postfix service is not active: %s\n", logsafe.Text(string(active)))
		return sysexits.TEMPFAIL
	}
	for _, item := range [][2]string{{"default_transport", "smtp2go-helper:"}, {"inet_interfaces", "loopback-only"}, {"mydestination", "localhost"}} {
		out, e := exec.Command("postconf", "-h", item[0]).Output()
		if e != nil || strings.TrimSpace(string(out)) != item[1] {
			fmt.Fprintf(stderr, "postfix %s: ERROR\n", item[0])
			return sysexits.TEMPFAIL
		}
		fmt.Fprintf(stderr, "postfix %s: %s\n", item[0], item[1])
	}
	for _, item := range [][2]string{{"relayhost", ""}, {"smtp_sasl_auth_enable", "no"}} {
		out, e := exec.Command("postconf", "-h", item[0]).Output()
		if e != nil || strings.TrimSpace(string(out)) != item[1] {
			fmt.Fprintf(stderr, "postfix %s must be %q\n", item[0], item[1])
			return sysexits.TEMPFAIL
		}
	}
	key, e := security.ReadAPIKey(security.DefaultKeyPath)
	if e != nil {
		fmt.Fprintf(stderr, "API key: ERROR (%s)\n", logsafe.Text(e.Error()))
		return sysexits.TEMPFAIL
	}
	_ = key
	fmt.Fprintln(stderr, "API key: configured and readable")
	ss, err := exec.Command("ss", "-lnt").Output()
	if err != nil {
		fmt.Fprintln(stderr, "SMTP listener: unable to inspect")
		return sysexits.TEMPFAIL
	}
	listeners := strings.Split(string(ss), "\n")
	for _, line := range listeners {
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.HasPrefix(fields[0], "LISTEN") && strings.HasSuffix(fields[3], ":25") && fields[3] != "127.0.0.1:25" && fields[3] != "[::1]:25" {
			fmt.Fprintln(stderr, "SMTP listener: EXTERNAL (unsafe)")
			return sysexits.TEMPFAIL
		}
	}
	fmt.Fprintln(stderr, "SMTP listener: no external port 25 listener detected")
	perms, e := smtp2go.New(cfg.Endpoint, cfg.Timeout(), key, cfg.FastAccept).Permissions(context.Background())
	if e != nil {
		fmt.Fprintf(stderr, "API permissions: unavailable (%s)\n", logsafe.Text(e.Error()))
		return sysexits.TEMPFAIL
	}
	fmt.Fprintf(stderr, "API /email/send permission: %s\n", perms.Send)
	if perms.Send != "allowed" {
		return sysexits.TEMPFAIL
	}
	return sysexits.OK
}
