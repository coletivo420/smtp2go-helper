# Migration from the Python `/email/mime` experiment

The production Python `postfix-smtp2go-api` implementation and `/email/mime` endpoint are deliberately abandoned. The new project starts from Go source and uses the upstream `smtp2go-go` models plus an HTTP adapter to `/email/send`.

## Safe sequence

1. Preserve the prior Postfix files, Python helper, protected legacy key path, Webmin state, and queue diagnostics in the root-only backup.
2. Build and run all offline Go tests. Install the new binary, service account, JSON config and protected key copy. Keep the Python helper and old account.
3. Stage Postfix `master.cf`/`main.cf` replacements; run `postfix check`. On error, restore the backup and do not reload.
4. Reload Postfix and confirm loopback-only listener, `default_transport=smtp2go-helper:`, recipient limit 1, and both historical IDs still held.
5. Submit exactly one real message via `/usr/sbin/sendmail`. If it fails, stop and restore the previous transport config; do not release/delete old held mail.
6. Only after successful acceptance may the active Python transport/helper/account be retired. Keep backups.

## Rollback

Run `scripts/migrate-from-smtp2go-api.sh --rollback BACKUP_DIR` as root. It restores only Postfix `main.cf` and `master.cf`, runs `postfix check`, then reloads if valid. It does not touch queue contents, the Python helper, either key copy, or backups. On failed validation it leaves the service untouched and reports the error.
