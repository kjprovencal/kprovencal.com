#!/usr/bin/env bash
# Exercises deploy/write-api-env.sh with fake values.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
script="$root/deploy/write-api-env.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

session='abcdefghijklmnopqrstuvwxyz012345'
admin='$2a$10$abcdefghijklmnopqrstuv0123456789'
deploy_path='/var/www/kprovencal'
marker='spaces-secret-marker'

base_env=(
  SESSION_SECRET="$session"
  ADMIN_PASSWORD_BCRYPT="$admin"
  DEPLOY_PATH="$deploy_path"
)

run_with() {
  local out=$1
  shift
  env -u TURNSTILE_SECRET_KEY -u CORS_ORIGIN -u LOG_LEVEL -u LOG_PATH \
    -u HOST_STATUS_PATH -u ADMIN_SESSION_HOURS -u RATE_LIMIT_GET_MAX \
    -u RATE_LIMIT_POST_MAX -u RATE_LIMIT_DISABLED -u SPACES_KEY \
    -u SPACES_SECRET -u SPACES_BUCKET -u SPACES_REGION -u SPACES_ENDPOINT \
    "$@" "$script" "$out"
}

out="$work/env"
stdout="$work/stdout"
stderr="$work/stderr"
run_with "$out" "${base_env[@]}" >"$stdout" 2>"$stderr"
[[ ! -s "$stdout" ]]
[[ ! -s "$stderr" ]]
[[ "$(stat -f '%Lp' "$out" 2>/dev/null || stat -c '%a' "$out")" == "600" ]]
grep -F -q "SESSION_SECRET='${session}'" "$out"
grep -F -q "ADMIN_PASSWORD_BCRYPT='${admin}'" "$out"
grep -F -q "BADGER_PATH='/var/www/kprovencal/data/badger'" "$out"
grep -F -q "APP_ENV='production'" "$out"
grep -F -q "TRUST_PROXY='1'" "$out"
grep -F -q "LISTEN_ADDR='127.0.0.1:8080'" "$out"
if grep -q '^SPACES_' "$out"; then
  echo "spaces keys were written without secrets" >&2
  exit 1
fi

run_with "$work/full" \
  "${base_env[@]}" \
  TURNSTILE_SECRET_KEY='turnstile-secret' \
  SPACES_KEY='spaces-key' \
  SPACES_SECRET="$marker" \
  SPACES_BUCKET='photos' \
  SPACES_REGION='nyc3' \
  SPACES_ENDPOINT='https://nyc3.digitaloceanspaces.com' \
  LOG_LEVEL='info' \
  >"$stdout"
grep -F -q "SPACES_SECRET='${marker}'" "$work/full"
grep -F -q "TURNSTILE_SECRET_KEY='turnstile-secret'" "$work/full"

set +e
run_with "$work/missing" SESSION_SECRET='' ADMIN_PASSWORD_BCRYPT="$admin" DEPLOY_PATH="$deploy_path" >"$stdout" 2>"$stderr"
status=$?
set -e
[[ $status -ne 0 ]]
[[ ! -e "$work/missing" ]]
grep -F -q 'SESSION_SECRET is required' "$stderr"
if grep -F -q "$admin" "$stderr"; then
  echo "stderr included the admin hash" >&2
  exit 1
fi

set +e
run_with "$work/partial" "${base_env[@]}" SPACES_KEY='only-key' >"$stdout" 2>"$stderr"
status=$?
set -e
[[ $status -ne 0 ]]
grep -F -q 'SPACES_SECRET' "$stderr"
[[ ! -e "$work/partial" ]]

set +e
run_with "$work/plain" "${base_env[@]}" ADMIN_PASSWORD_BCRYPT='not-a-hash' >"$stdout" 2>"$stderr"
status=$?
set -e
[[ $status -ne 0 ]]
grep -F -q 'bcrypt' "$stderr"

set +e
SESSION_SECRET="$session" ADMIN_PASSWORD_BCRYPT="$admin" DEPLOY_PATH="$deploy_path" \
  bash -x "$script" "$work/traced" >"$stdout" 2>"$stderr"
status=$?
set -e
[[ $status -ne 0 ]]
[[ ! -e "$work/traced" ]]
if grep -F -q "$session" "$stdout" "$stderr" || grep -F -q "$admin" "$stdout" "$stderr"; then
  echo "xtrace run printed a secret" >&2
  exit 1
fi

echo "write-api-env tests passed"
