# Architecture

Local programs use the established sendmail interface. Postfix owns queueing, per-recipient delivery attempts, retry timing, and local logs. Its `smtp2go-helper` pipe invokes the Go program as the unprivileged service user, passes sender, one recipient, and queue ID as argv, and streams the original RFC message on stdin. No HTTP daemon or local SMTP listener is added.

The Go path is `config + key -> envelope validation -> MIME parser -> internal message model -> SMTP2GO SDK Email model -> bounded HTTPS adapter -> response classifier`. The SDK is used for canonical JSON field names and attachment/header types, not its `Send()` routine. The adapter preserves HTTP status and SMTP2GO response fields and has an explicit timeout.

The remote API only receives the current envelope recipient. MIME `To`, `Cc`, and `Bcc` are excluded from recipient selection and are not sent as active API recipients. `From` is selected from a valid MIME From, then the nonempty valid envelope sender, then configured default. Null reverse-path is supported for DSNs.

Postfix is send-only: `inet_interfaces=loopback-only`, `mydestination=localhost`, `default_transport=smtp2go-helper:`, and recipient limit 1. There is no SMTP AUTH, SMTP relay, `/email/mime`, mailbox service, or local DKIM signing.
