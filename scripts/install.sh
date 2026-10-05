#!/bin/bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "$0")/.." && pwd)
install_webmin=0
[[ ${1:-} == --webmin ]] && install_webmin=1
[[ $EUID -eq 0 ]] || { echo 'run as root' >&2; exit 1; }
grep -q '^ID=debian' /etc/os-release || { echo 'Debian is required' >&2; exit 1; }
command -v go >/dev/null || { echo 'Go 1.24+ is required to build' >&2; exit 1; }
goversion=$(go env GOVERSION | sed 's/^go//')
[[ $(printf '%s\n' 1.24 "$goversion" | sort -V | head -n1) == 1.24 ]] || { echo 'Go 1.24+ is required' >&2; exit 1; }
command -v postfix >/dev/null || { echo 'Postfix is required' >&2; exit 1; }
(cd "$ROOT" && go test ./... && go vet ./...)
if ! getent group smtp2go-helper >/dev/null; then groupadd --system smtp2go-helper; fi
if ! getent passwd smtp2go-helper >/dev/null; then useradd --system --gid smtp2go-helper --home-dir /nonexistent --no-create-home --shell /usr/sbin/nologin smtp2go-helper; fi
install -d -o root -g root -m 0755 /usr/local/libexec
build=$(mktemp)
trap 'rm -f "$build"' EXIT
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$build" ./cmd/smtp2go-helper)
install -o root -g root -m 0755 "$build" /usr/local/libexec/smtp2go-helper
install -d -o root -g smtp2go-helper -m 0750 /etc/smtp2go-helper
if [[ ! -e /etc/smtp2go-helper/config.json ]]; then
  install -o root -g smtp2go-helper -m 0640 "$ROOT/packaging/config.json.example" /etc/smtp2go-helper/config.json
fi
if [[ ! -e /etc/smtp2go-helper/api.key && -f /etc/smtp2go/api.key ]]; then
  install -o root -g smtp2go-helper -m 0640 /etc/smtp2go/api.key /etc/smtp2go-helper/api.key
fi
if [[ -e /etc/smtp2go-helper/api.key ]]; then chown root:smtp2go-helper /etc/smtp2go-helper/api.key; chmod 0640 /etc/smtp2go-helper/api.key; fi
chown root:smtp2go-helper /etc/smtp2go-helper/config.json
chmod 0640 /etc/smtp2go-helper/config.json
/usr/local/libexec/smtp2go-helper --version
/usr/local/libexec/smtp2go-helper config validate
if (( install_webmin )); then
  webroot=$(awk -F= '$1=="root" {print $2; exit}' /etc/webmin/miniserv.conf 2>/dev/null || true)
  [[ -n $webroot && -d $webroot ]] || { echo 'Cannot discover Webmin root from miniserv.conf' >&2; exit 1; }
  install -d -o root -g root -m 0755 "$webroot/smtp2go-helper"
  cp -a "$ROOT/webmin/smtp2go-helper/." "$webroot/smtp2go-helper/"
  chown -R root:root "$webroot/smtp2go-helper"
  python3 - <<'PY'
from pathlib import Path
import os, tempfile
p=Path('/etc/webmin/webmin.acl')
lines=p.read_text().splitlines()
found=False
for i,line in enumerate(lines):
    if line.startswith('root:'):
        modules=line.split(':',1)[1].split()
        if 'smtp2go-helper' not in modules:
            lines[i]='root: '+' '.join(modules+['smtp2go-helper'])
        found=True
        break
if not found:
    raise SystemExit('Webmin root ACL entry not found; module files installed but access was not granted')
st=p.stat()
fd,tmp=tempfile.mkstemp(prefix='.webmin.acl.',dir=str(p.parent))
try:
    with os.fdopen(fd,'w') as f:
        f.write('\n'.join(lines)+'\n')
    os.chmod(tmp,st.st_mode & 0o777)
    os.chown(tmp,st.st_uid,st.st_gid)
    os.replace(tmp,p)
finally:
    if os.path.exists(tmp): os.unlink(tmp)
PY
  install -d -o root -g root -m 0750 /etc/webmin/smtp2go-helper
  cat > /etc/webmin/smtp2go-helper/config <<EOF
helper_bin=/usr/local/libexec/smtp2go-helper
config_file=/etc/smtp2go-helper/config.json
key_file=/etc/smtp2go-helper/api.key
postfix_bin=/usr/sbin/postfix
sendmail_bin=/usr/sbin/sendmail
EOF
  chmod 0600 /etc/webmin/smtp2go-helper/config
  systemctl restart webmin
  systemctl is-active --quiet webmin || { echo 'Webmin did not return active after module installation' >&2; exit 1; }
fi
echo 'Helper installed. Postfix routing has not been changed.'
