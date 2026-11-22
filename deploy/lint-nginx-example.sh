#!/usr/bin/env bash
# Syntax-check the example nginx site together with its rate-limit fragment.
set -euo pipefail

if ! command -v nginx >/dev/null 2>&1; then
  echo "nginx is not installed; skipping example config test" >&2
  exit 0
fi

root=$(cd "$(dirname "$0")/.." && pwd)
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
mkdir -p "$dir/www" "$dir/body" "$dir/proxy" "$dir/fastcgi" "$dir/uwsgi" "$dir/scgi"

sed -e "s#/var/www/kprovencal/dist#$dir/www#g" -e "s/listen 80;/listen 127.0.0.1:18080;/" "$root/deploy/nginx-site.conf.example" > "$dir/site.conf"
cp "$root/deploy/nginx-http-rate-limits.conf.example" "$dir/rate.conf"
cat > "$dir/nginx.conf" <<EOF
pid $dir/nginx.pid;
error_log $dir/error.log;
events {}
http {
    access_log off;
    client_body_temp_path $dir/body;
    proxy_temp_path $dir/proxy;
    fastcgi_temp_path $dir/fastcgi;
    uwsgi_temp_path $dir/uwsgi;
    scgi_temp_path $dir/scgi;
    include $dir/rate.conf;
    include $dir/site.conf;
}
EOF

nginx -t -c "$dir/nginx.conf" -p "$dir"
