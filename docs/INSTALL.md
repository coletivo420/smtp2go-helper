# Installation (Debian)

Build and validate the source with Go 1.24+: `scripts/validate.sh`. As root, run `scripts/install.sh`; it builds with `CGO_ENABLED=0`, creates the locked system account, and installs `/usr/local/libexec/smtp2go-helper`. It preserves existing configuration and key files. It does not install packages or route production Postfix traffic.

Create `/etc/smtp2go-helper/config.json` from `packaging/config.json.example` if absent. Configure `/etc/smtp2go-helper/api.key` with owner `root:smtp2go-helper`, mode 0640, and directory mode 0750. The installer migrates an existing `/etc/smtp2go/api.key` without displaying it and never puts it in source, argv, or persistent environment.

Check `smtp2go-helper --version`, `smtp2go-helper config validate`, then run `scripts/migrate-from-smtp2go-api.sh` as root. That script takes a root-only backup, tests before activation, changes only Postfix transport lines, runs `postfix check`, and reloads only after validation. Keep the old helper and the held messages until the single real send succeeds.

Optional Webmin installation is performed by `scripts/install.sh --webmin`; the module path is discovered from the active Webmin installation. Review [rollback](MIGRATION.md#rollback) before starting.
