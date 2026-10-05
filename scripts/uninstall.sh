#!/bin/bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'run as root' >&2; exit 1; }
[[ ${1:-} == --restore ]] || { echo 'Usage: uninstall.sh --restore BACKUP_DIR; queue, key, binary and backups are preserved.' >&2; exit 2; }
backup=${2:?missing backup directory}
[[ -d $backup/etc-postfix ]] || { echo 'invalid backup directory' >&2; exit 1; }
cp -a "$backup/etc-postfix/main.cf" /etc/postfix/main.cf
cp -a "$backup/etc-postfix/master.cf" /etc/postfix/master.cf
postfix check && postfix reload
echo 'Postfix restored. Queue, API key, binary, service account and backups were preserved.'
