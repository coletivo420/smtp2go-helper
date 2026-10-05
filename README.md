# SMTP2GO Helper

**SMTP2GO Helper** is a Go send-only transport for Postfix. Local applications submit through `/usr/sbin/sendmail`; Postfix queues each envelope recipient separately and invokes this helper, which maps the MIME message to SMTP2GO's `/v3/email/send` JSON API over verified HTTPS.

```text
PHP / WordPress / cron / Webmin -> sendmail -> Postfix queue
  -> smtp2go-helper pipe (one envelope recipient) -> /v3/email/send -> SMTP2GO
```

The helper uses upstream [`github.com/smtp2go-oss/smtp2go-go`](https://github.com/smtp2go-oss/smtp2go-go) data structures. It deliberately uses a small local HTTP adapter because the upstream `Send` implementation uses an unbounded `http.Client`, reads its key from an environment variable, and does not expose enough response state for safe MTA queue decisions. See [SMTP2GO API behavior](docs/SMTP2GO.md).

## Requirements

- Debian 13 or compatible Linux, Postfix 3.8+, Go 1.24+ to build (no Go runtime needed).
- A SMTP2GO API key allowed to call `/email/send` and a verified sender domain.
- Root for installation; runtime service account `smtp2go-helper`.

## Build and test

```sh
go mod tidy
gofmt -w cmd internal
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o build/smtp2go-helper ./cmd/smtp2go-helper
```

## Install

See [installation](docs/INSTALL.md), [Postfix integration](docs/POSTFIX.md), and [Webmin module](docs/WEBMIN.md). The migration script takes a root-only backup, validates first, and retains the prior transport/helper for rollback.

The key belongs in `/etc/smtp2go-helper/api.key` (`root:smtp2go-helper`, `0640`); configuration is `/etc/smtp2go-helper/config.json` (`root:smtp2go-helper`, `0640`). Neither belongs in Git. No key is passed in argv or a persistent environment variable.

## Operational behavior

Version 0.1.1 adds bounded MIME parsing, strict API-key/config metadata checks,
restricted HTTP destinations with redirect refusal, bounded API responses,
POST-only Webmin mutations, race testing, and pinned vulnerability scanning.
The system install still requires the service account to read only the protected
key file; see [security design](docs/SECURITY.md).

- `smtp2go-helper --version`
- `smtp2go-helper config validate`
- `smtp2go-helper doctor` (read-only; checks Postfix and queries API-key endpoint permissions)
- `smtp2go-helper api permissions` (read-only)
- Postfix retries infrastructure/configuration uncertainty (`EX_TEMPFAIL`, 75). Clearly invalid message data is permanent (`EX_DATAERR`, 65).
- Only the current Postfix envelope recipient is submitted. Original `To`, `Cc`, and `Bcc` do not control delivery.
- SMTP AUTH, SMTP relay, `/email/mime`, local DKIM, and externally listening SMTP are not used.

## License

This project is GPL-3.0-or-later. The upstream SMTP2GO Go SDK remains under its own MIT license; it is consumed as a Go module and is not copied into this repository. See [license notes](docs/DEVELOPMENT.md).
