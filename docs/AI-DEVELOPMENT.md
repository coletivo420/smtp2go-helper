# AI and maintainer invariants

- Core implementation is Go; module path is `github.com/coletivo420/smtp2go-helper`.
- Upstream SDK is `github.com/smtp2go-oss/smtp2go-go`; use its models, do not fork or copy it.
- Only endpoint is `/v3/email/send`. `/email/mime` is retired and must not return.
- No SMTP AUTH, SASL, SMTP2GO `relayhost`, or local DKIM signing.
- Recipient limit equals one; each API `to` list contains only the exact Postfix envelope recipient. MIME To/Cc/Bcc are never used for delivery.
- Postfix owns queue/retry and remains loopback-only. The pipe helper is an unprivileged system user.
- Preserve empty envelope sender (DSN) and apply sender priority MIME From, envelope sender, configured default.
- `fastaccept` is initially false. Never treat HTTP 200 alone as acceptance.
- Infrastructure, credential permissions, API authorization, ambiguous results, timeout, 429 and 5xx are temporary failures (75). Only clear message-specific errors are permanent (65).
- API key never enters Git, argv, persistent environment, config JSON, logs, or UI response.
- DKIM-Signature input is not forwarded; SMTP2GO Sender Domain signs the final outgoing message.
- Webmin test submits through `/usr/sbin/sendmail`; queue actions require their own ACL and confirmation.
- No implementation task may release or delete existing held messages without an explicit request.
- Update docs and regression tests when changing any item above.
