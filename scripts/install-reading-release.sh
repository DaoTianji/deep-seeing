#!/usr/bin/env bash
set -euo pipefail
release=${1:?release hash required}
[[ "$release" =~ ^[a-f0-9]{12}$ ]] || exit 2
incoming=/tmp/shuzhongjian-release-20260914
target=/opt/shuzhongjian/releases/$release
test -f "$incoming/reading"
test -f "$incoming/model.env"
test ! -e "$target"
id shuzhongjian >/dev/null 2>&1 || useradd --system --home-dir /var/lib/shuzhongjian --shell /usr/sbin/nologin shuzhongjian
install -d -m 0755 /opt/shuzhongjian /opt/shuzhongjian/releases "$target"
install -d -o shuzhongjian -g shuzhongjian -m 0700 /var/lib/shuzhongjian /var/lib/shuzhongjian/data
install -d -o root -g shuzhongjian -m 0750 /etc/shuzhongjian
install -m 0755 "$incoming/reading" "$target/reading"
if test -f /etc/shuzhongjian/model.env; then
 echo 'Keeping existing model-only configuration'
else
 install -o root -g shuzhongjian -m 0640 "$incoming/model.env" /etc/shuzhongjian/model.env
fi
install -m 0644 "$incoming/shuzhongjian.service" /etc/systemd/system/shuzhongjian.service
ln -s "$target" /opt/shuzhongjian/current.next
mv -Tf /opt/shuzhongjian/current.next /opt/shuzhongjian/current
systemctl daemon-reload
systemctl enable --now shuzhongjian
systemctl restart shuzhongjian
systemctl is-active shuzhongjian
sha256sum "$target/reading"
