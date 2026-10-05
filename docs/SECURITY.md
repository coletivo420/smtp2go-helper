# Security model

**Secrets.** API key file is a regular non-symlink file, mode 0640, owner root, group `smtp2go-helper`; parent is installed root:service-group 0750. The process receives it in memory. It is not an argument, config value, persistent environment variable, command history item, log field, or repository file. Key diagnostics reveal only presence/permissions.

**Privileges and network.** Postfix runs the pipe under a locked system user, not root. The binary writes no system configuration and starts no listener. HTTPS uses Go's normal system trust roots and hostname verification. SMTP service listens only on IPv4/IPv6 loopback.

**Data handling.** MIME and API payload are never logged. Logs are short status records with sanitized API request/email IDs and queue ID. Header names and values are allowlisted and reject CR/LF/NUL injection. Recipient comes only from Postfix envelope.

**Queue safety.** Timeouts, DNS/network errors, 408/425/429, 5xx, auth/permission/configuration errors, invalid JSON and unknown API outcomes use temporary failure code 75. Clearly malformed message/envelope data uses code 65. An API success is returned only when response fields indicate acceptance and no failed recipients.

**Webmin.** Webmin is privileged; module ACLs gate view, configure, key replacement, test send, queue view/change, and Postfix reload separately. The UI never renders the key. Queue actions validate queue IDs and require explicit confirmation. Test email calls local sendmail, never the API directly.

**Bounds and files.** The helper opens key/config files without following symlinks and validates regular-file type, root/service ownership, exact modes, key format, and bounded size. MIME has documented limits: 10,240,000 message bytes, 64 KiB headers, 200 headers, 16 KiB header lines, 256 MIME parts, depth 20, 32 attachments and 32 inlines, 8 MiB per binary part, and 9 MiB total decoded binary data. API responses are limited to 1 MiB. Exceeding a message-specific structural bound returns a permanent data error; network/unknown outcomes remain temporary.

**HTTP destination.** Authenticated requests are restricted to `https://api.smtp2go.com` and the `/v3/email/send` or `/v3/api_keys/permissions` paths. Redirects are not followed, so the key and message cannot be forwarded to a redirect target.

**Webmin writes.** Mutating handlers require POST and their specific ACL. The module uses Webmin `init_config()` referer validation, list-form command execution, escaped output, and exclusive same-directory temporary files followed by atomic rename. It does not render MIME/body content.
