# Troubleshooting

- `config invalid`: run `smtp2go-helper config validate`; configuration is strict JSON and unknown fields fail.
- `API key unavailable`: inspect metadata only with `stat`; expected root:service group, 0640, valid `api-` token format. Do not print the file.
- `HTTP 401/403` or endpoint permission: helper returns 75 so mail stays queued. Check key permissions for `/email/send` in SMTP2GO; do not retry-flush in a loop.
- HTTP 429/5xx, timeout, DNS/TLS issue: temporary failure; inspect `journalctl -u postfix` and queue. Fix the cause and let Postfix retry.
- HTTP 400: response classification includes only bounded/sanitized diagnostic fields. Message-specific recipient/payload failures are permanent; API configuration/permission errors are temporary.
- `doctor`: read-only checks local Postfix transport/listener/config and calls the API key permission endpoint, but does not send email.
- For MIME issues, reproduce with a fixture and `go test ./internal/mimeparser`; never add raw message content to logs.
- `MIME nesting too deep`, `too many MIME parts`, or attachment-limit errors are message-specific permanent data failures. Limits are intentionally bounded to protect the transport process; reduce the message payload rather than raising them without review.
- An API redirect is deliberately not followed. Check the configured official endpoint and SMTP2GO service status; the helper will retain the message for retry.

Useful commands: `postqueue -p`, `postfix check`, `postconf -n`, `postconf -M smtp2go-helper`, `systemctl status postfix`, `journalctl -u postfix`.
