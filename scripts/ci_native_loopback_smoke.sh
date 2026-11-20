#!/usr/bin/env bash
# Synthetic local-only native Gateway smoke. No public listeners or production data.
set -euo pipefail
binary="${GATEWAY_SMOKE_BINARY:-./gateway}"
expected="${GATEWAY_SMOKE_EXPECTED_VERSION:-0.1.0-ci-arm64}"
if [[ ! -x "$binary" ]]; then
  printf 'Gateway binary is not executable: %s\n' "$binary" >&2
  exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/goreecloud-gateway-arm-smoke-XXXXXXXX")"
chmod 0700 "$work"
pid=''
cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf -- "$work"
}
trap cleanup EXIT INT TERM

# Allocate an ephemeral loopback port. The socket closes before Gateway binds.
port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
token="$(openssl rand -hex 32)"
GATEWAY_ADMIN_TOKEN="$token" "$binary" serve \
  --listen "127.0.0.1:$port" \
  --sites "$work/sites.json" \
  --state-dir "$work/state" > "$work/console.log" 2>&1 &
pid="$!"

ready=0
for attempt in {1..25}; do
  if curl --noproxy '*' -fsS --max-time 2 "http://127.0.0.1:$port/api/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo 'Gateway exited before readiness' >&2
    exit 1
  fi
  sleep 0.25
done
if [[ "$ready" != 1 ]]; then
  echo 'Gateway loopback health endpoint unavailable' >&2
  exit 1
fi

status="$(curl --noproxy '*' -fsS --max-time 3 \
  -H "Authorization: Bearer $token" \
  "http://127.0.0.1:$port/api/status")"
python3 -c 'import json,sys; s=json.loads(sys.stdin.read()); expected=sys.argv[1]; assert s["version"] == expected and s["management"] == "loopback" and s["engine"] == "Caddy v2" and s["writable"] is False and s["siteCount"] == 0, s; print("Authorized loopback and version: OK")' "$expected" <<< "$status"

unauthed="$(curl --noproxy '*' -sS --max-time 3 -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:$port/api/status")"
bad_host="$(curl --noproxy '*' -sS --max-time 3 -o /dev/null -w '%{http_code}' \
  -H 'Host: invalid.example' -H "Authorization: Bearer $token" \
  "http://127.0.0.1:$port/api/status")"
read_only="$(curl --noproxy '*' -sS --max-time 3 -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  --data '{"host":"example.test","upstream":"http://127.0.0.1:1234"}' \
  "http://127.0.0.1:$port/api/sites")"

# Exercise the authenticated, non-mutating preview against the actual native
# executable. No hostnames are resolved and no proxy listeners are created.
preview="$(curl --noproxy '*' -fsS --max-time 3 \
  -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  --data '{"host":"preview.example.test","upstream":"http://127.0.0.1:1234"}' \
  "http://127.0.0.1:$port/api/validate-site")"
python3 -c 'import json,sys; p=json.loads(sys.stdin.read()); assert p["valid"] is True and p["dryRun"] is True and p["applied"] is False and p["site"]["host"] == "preview.example.test" and p["site"]["upstream"] == "http://127.0.0.1:1234"; print("Native non-mutating route preview: OK")' <<< "$preview"

unauth_preview="$(curl --noproxy '*' -sS --max-time 3 -o /dev/null -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  --data '{"host":"preview.example.test","upstream":"http://127.0.0.1:1234"}' \
  "http://127.0.0.1:$port/api/validate-site")"
duplicate_preview="$(curl --noproxy '*' -sS --max-time 3 -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
  --data '{"Host":"preview.example.test","host":"other.example.test","upstream":"http://127.0.0.1:1234"}' \
  "http://127.0.0.1:$port/api/validate-site")"
sites="$(curl --noproxy '*' -fsS --max-time 3 -H "Authorization: Bearer $token" \
  "http://127.0.0.1:$port/api/sites")"
python3 -c 'import json,sys; assert json.loads(sys.stdin.read())["sites"] == []' <<< "$sites"
[[ ! -e "$work/sites.json" ]] || { echo 'Preview unexpectedly persisted a managed route' >&2; exit 1; }
[[ "$unauth_preview" == 401 ]] || { echo "Unauthenticated preview accepted: $unauth_preview" >&2; exit 1; }
[[ "$duplicate_preview" == 400 ]] || { echo "Ambiguous preview accepted: $duplicate_preview" >&2; exit 1; }

[[ "$unauthed" == 401 ]] || { echo "Unexpected unauthenticated status: $unauthed" >&2; exit 1; }
[[ "$bad_host" == 403 ]] || { echo "Unexpected spoofed Host status: $bad_host" >&2; exit 1; }
[[ "$read_only" == 403 ]] || { echo "Unexpected write status: $read_only" >&2; exit 1; }
[[ "$(stat -c %a "$work/state")" == 700 ]] || {
  echo 'Managed state is not private mode 0700' >&2
  exit 1
}
echo 'Native loopback smoke: PASS (no production ingress, DNS, TLS or credentials)'
