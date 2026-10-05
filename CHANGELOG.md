# Changelog

All notable changes follow Semantic Versioning.

## 0.1.1 - 2026-10-05

- Harden API-key/config file checks against symlinks, wrong ownership/modes, invalid content, and oversized files.
- Add bounded MIME header/part/depth/attachment parsing and malformed-encoding checks.
- Restrict authenticated HTTP requests to the official SMTP2GO HTTPS endpoints, reject redirects, and bound response bodies.
- Harden Webmin privileged operations with POST-only actions, per-operation ACL checks, safer atomic writes, and protected key reads.
- Add CI race detection and pinned `govulncheck` v1.8.0; pin GitHub Actions by commit SHA.
- Retire the experimental SMTP2GO API/MIME runtime and unused mail-protocol firewall exposure after operational checks.
- Validate PHP `mail()` and a synthetic multipart/inline/attachment delivery through Postfix.
- Expand security, MIME, HTTP, and operational regression tests and documentation.
- Correct LF/CRLF separator selection, retain sanitized log windows, stream queue counts, and synchronize Debian/Webmin version metadata.

## 0.1.0 - 2026-10-05

- Initial Go helper using SMTP2GO `/v3/email/send` and upstream `smtp2go-go` models.
- Single-recipient Postfix pipe transport, MIME parser, safe retries, Debian scripts, and Webmin administration module.
