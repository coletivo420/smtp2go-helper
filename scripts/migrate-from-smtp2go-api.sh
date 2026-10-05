#!/bin/bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "$0")/.." && pwd)
[[ $EUID -eq 0 ]] || { echo 'run as root' >&2; exit 1; }
if [[ ${1:-} == --rollback ]]; then
  backup=${2:?usage: migrate-from-smtp2go-api.sh --rollback BACKUP_DIR}
  [[ -d $backup/etc-postfix ]] || { echo 'backup lacks etc-postfix' >&2; exit 1; }
  cp -a "$backup/etc-postfix/main.cf" /etc/postfix/main.cf
  cp -a "$backup/etc-postfix/master.cf" /etc/postfix/master.cf
  postfix check && postfix reload
  echo 'Postfix restored; queue and helper files were not changed.'
  exit 0
fi
command -v go >/dev/null || { echo 'Go 1.24+ is required' >&2; exit 1; }
command -v postconf >/dev/null
postfix check
current=$(postconf -h default_transport)
[[ $current == smtp2go-api: || $current == smtp2go-helper: ]] || { echo "Unexpected default_transport: $current" >&2; exit 1; }
postconf -h inet_interfaces | grep -qx loopback-only || { echo 'Postfix is not loopback-only' >&2; exit 1; }
[[ -z $(postconf -h relayhost) ]] || { echo 'Unexpected SMTP relayhost is configured; stop and review before migration' >&2; exit 1; }
[[ $(postconf -h smtp_sasl_auth_enable) == no ]] || { echo 'SMTP AUTH is enabled; stop and review before migration' >&2; exit 1; }
[[ -z $(postconf -h smtp_sasl_password_maps) ]] || { echo 'SMTP SASL password map remains configured; stop and review before migration' >&2; exit 1; }
for id in 049542059A 05C7220595; do postqueue -p | grep -F "${id}!" >/dev/null || { echo "Expected held queue entry $id not found; stopping" >&2; exit 1; }; done
(cd "$ROOT" && ./scripts/validate.sh)
stamp=$(date -u +%Y%m%d-%H%M%S)
backup=/root/backup-pre-smtp2go-helper-go-$stamp
install -d -o root -g root -m 0700 "$backup"
cp -a /etc/postfix "$backup/etc-postfix"
[[ ! -e /usr/local/libexec/postfix-smtp2go-api ]] || cp -a /usr/local/libexec/postfix-smtp2go-api "$backup/"
[[ ! -d /etc/smtp2go ]] || cp -a /etc/smtp2go "$backup/"
if [[ -d /etc/webmin ]]; then install -d -m 0700 "$backup/etc-webmin"; cp -a /etc/webmin/postfix "$backup/etc-webmin/" 2>/dev/null || true; cp -a /etc/webmin/smtp2go-helper "$backup/etc-webmin/" 2>/dev/null || true; fi
postconf -n > "$backup/postconf-n.txt"
postconf -M > "$backup/postconf-M.txt"
postqueue -p > "$backup/postqueue.txt"
ss -lntp > "$backup/ss-lntp.txt"
firewall-cmd --list-all > "$backup/firewalld.txt"
printf '%s\n' 'Pre-migration route: Python smtp2go-api pipe using /email/mime. Existing held messages must remain untouched.' > "$backup/STATE-BEFORE.txt"
chmod -R go-rwx "$backup"
if [[ ${1:-} == --webmin ]]; then "$ROOT/scripts/install.sh" --webmin; else "$ROOT/scripts/install.sh"; fi
python3 - <<'PY'
from pathlib import Path
p=Path('/etc/postfix/master.cf')
lines=p.read_text().splitlines()
out=[]
skip_continuations=False
for line in lines:
    if line.startswith('smtp2go-api '):
        skip_continuations=True
        continue
    if skip_continuations and line[:1].isspace():
        continue
    skip_continuations=False
    out.append(line)
out += ['', 'smtp2go-helper unix - n n - 1 pipe',
        '  flags=q user=smtp2go-helper:smtp2go-helper null_sender= argv=/usr/local/libexec/smtp2go-helper ${sender} ${recipient} ${queue_id}']
tmp=p.with_name('master.cf.smtp2go-helper.tmp')
tmp.write_text('\n'.join(out)+'\n')
tmp.chmod(0o644)
tmp.replace(p)
PY
postconf -X smtp2go-api_destination_recipient_limit
postconf -e 'default_transport = smtp2go-helper:'
postconf -e 'smtp2go-helper_destination_recipient_limit = 1'
postconf -e 'message_size_limit = 10240000'
if ! postfix check; then
  cp -a "$backup/etc-postfix/main.cf" /etc/postfix/main.cf
  cp -a "$backup/etc-postfix/master.cf" /etc/postfix/master.cf
  postfix check || true
  echo "Validation failed; prior config restored from $backup. No reload performed." >&2
  exit 1
fi
if ! postfix reload; then
  cp -a "$backup/etc-postfix/main.cf" /etc/postfix/main.cf
  cp -a "$backup/etc-postfix/master.cf" /etc/postfix/master.cf
  postfix check && postfix reload || true
  echo "Reload failed; prior config restored from $backup." >&2
  exit 1
fi
postconf -h default_transport
postconf -h smtp2go-helper_destination_recipient_limit
postconf -M smtp2go-helper
ss -lntp
postfix check
for id in 049542059A 05C7220595; do postqueue -p | grep -F "${id}!" >/dev/null || { echo "Held queue entry $id changed unexpectedly" >&2; exit 1; }; done
echo "Migration config activated. Backup: $backup. Old helper retained. Run only the specifically authorized single end-to-end send test before cleanup."
