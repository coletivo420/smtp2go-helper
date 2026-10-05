# Security policy

Report vulnerabilities privately to the repository maintainers; do not include live API keys, complete messages, recipients, or attachments in public reports. Rotate an exposed SMTP2GO key in the SMTP2GO account.

The helper runs as the unprivileged `smtp2go-helper` system account. The API key is a root-owned `0640` file readable by that service group. HTTPS certificate and hostname validation remain enabled. Keys are never put in argv, durable environment variables, config JSON, HTTP diagnostics, or logs. Message bodies, MIME, HTML, attachments, and API payloads are never logged.

The runtime rejects symlink/non-regular key and config files, checks exact owner/group/mode, limits config and API response sizes, refuses HTTP redirects, and restricts authenticated requests to the official SMTP2GO API host and documented endpoints. MIME parsing has explicit header, part, nesting, attachment, and binary-byte ceilings. Webmin mutations require POST and operation-specific ACLs.

Postfix accepts SMTP only on loopback. The Postfix queue is the retry authority. Unknown API outcomes are temporary failures to avoid silently losing mail. See [security design](docs/SECURITY.md).
