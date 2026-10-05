# Security model

**Secrets.** API key file is a regular non-symlink file, mode 0640, owner root, group `smtp2go-helper`; parent is installed root:service-group 0750. The process receives it in memory. It is not an argument, config value, persistent environment variable, command history item, log field, or repository file. Key diagnostics reveal only presence/permissions.

**Privileges and network.** Postfix runs the pipe under a locked system user, not root. The binary writes no system configuration and starts no listener. HTTPS uses Go's normal system trust roots and hostname verification. SMTP service listens only on IPv4/IPv6 loopback.

**Data handling.** MIME and API payload are never logged. Logs are short status records with sanitized API request/email IDs and queue ID. Header names and values are allowlisted and reject CR/LF/NUL injection. Recipient comes only from Postfix envelope.

**Queue safety.** Timeouts, DNS/network errors, 408/425/429, 5xx, auth/permission/configuration errors, invalid JSON and unknown API outcomes use temporary failure code 75. Clearly malformed message/envelope data uses code 65. An API success is returned only when response fields indicate acceptance and no failed recipients.

**Webmin.** Webmin is privileged; module ACLs gate view, configure, key replacement, test send, queue view/change, and Postfix reload separately. The UI never renders the key. Queue actions validate queue IDs and require explicit confirmation. Test email calls local sendmail, never the API directly.
