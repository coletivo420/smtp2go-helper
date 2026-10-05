# Webmin module

The native Perl/CGI module is copied to the Webmin root discovered from miniserv.conf; the installer does not assume a fixed root. It provides a dashboard, status/API-permission check, strict JSON configuration form, write-only API key replacement, local sendmail test, queue metadata/actions, and recent Postfix logs. Portuguese and English strings are included.

The test page submits a minimal message to /usr/sbin/sendmail; it never calls the API directly. Key replacement writes an atomic root-owned 0640 file and asks the helper to check /email/send; on failure it restores the prior file without rendering the key. The dashboard shows only configured/not configured and a short SHA-256 fingerprint.

Module ACL fields are view, configure, replace_api_key, test, queue_view, queue_modify, and reload_postfix. Queue modification requires a valid 10-character Postfix queue ID, the queue-modify ACL, and a confirmation checkbox. Message bodies are not shown. The module does not enable mail hosting, IMAP, POP, or external SMTP listeners.

Install/update with scripts/install.sh --webmin. The installer discovers the Webmin root and adds the module to the existing root user's module ACL, then restarts Webmin. No other Webmin users are granted access automatically. Inspect systemctl status webmin afterward. A syntax check alone is not a browser-session check; confirm the module is listed and dashboard opens in an authenticated Webmin session.
