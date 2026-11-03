#!/usr/bin/env bash
# Isolated, development-only integration of the Gateway engine and console.
# No real domains, ACME requests, credentials, privileged ports or live config.
set -euo pipefail
umask 077
cd "$(dirname "$0")/.."
test -x ./gateway
test -x ./gateway-console
for tool in openssl curl python3; do command -v "$tool" >/dev/null; done

python3 - <<'PY'
import socket
for port in (2019, 7689, 8080):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.25)
        if sock.connect_ex(("127.0.0.1", port)) == 0:
            raise SystemExit("Test port is already in use")
PY

scratch="$(mktemp -d)"
engine_pid=""
console_pid=""
finish() {
    if [[ -n "$console_pid" ]]; then
        kill "$console_pid" >/dev/null 2>&1 || true
        wait "$console_pid" >/dev/null 2>&1 || true
    fi
    if [[ -n "$engine_pid" ]]; then
        kill "$engine_pid" >/dev/null 2>&1 || true
        wait "$engine_pid" >/dev/null 2>&1 || true
    fi
    rm -rf -- "$scratch"
}
trap finish EXIT
token="$(openssl rand -hex 32)"

./gateway run --config configs/Gatewayfile.example --adapter caddyfile \
    > "$scratch/engine.log" 2>&1 &
engine_pid=$!
engine_ready=0
for _ in $(seq 1 100); do
    if curl --silent --fail --max-time 1 -o /dev/null \
        http://127.0.0.1:8080/ &&
       curl --silent --fail --max-time 1 -o /dev/null \
        http://127.0.0.1:2019/config/; then
        engine_ready=1
        break
    fi
    sleep 0.2
done
if [[ "$engine_ready" != 1 ]]; then
    echo "Isolated Gateway engine did not become ready" >&2
    exit 1
fi

GOREE_GATEWAY_CONSOLE_TOKEN="$token" ./gateway-console \
    > "$scratch/console.log" 2>&1 &
console_pid=$!
console_ready=0
for _ in $(seq 1 100); do
    if curl --silent --fail --max-time 1 -o /dev/null \
        http://127.0.0.1:7689/healthz; then
        console_ready=1
        break
    fi
    sleep 0.2
done
if [[ "$console_ready" != 1 ]]; then
    echo "Isolated Gateway console did not become ready" >&2
    exit 1
fi

api="http://127.0.0.1:7689/api/v1/overview"
http_code() {
    curl --silent --show-error --max-time 3 \
        --output /dev/null --write-out '%{http_code}' "$@"
}
test "$(http_code "$api")" = 401
test "$(http_code -X POST "$api")" = 405

# The first missing token counted as a failure. All forwarded IPs are forged.
for i in $(seq 1 7); do
    test "$(http_code -H 'Authorization: Bearer incorrect' \
        -H "X-Forwarded-For: 198.51.100.$i" "$api")" = 401
done
test "$(http_code -H 'Authorization: Bearer incorrect' \
    -H 'X-Forwarded-For: 203.0.113.222' "$api")" = 429

# Valid access must survive the invalid-token throttle.
curl --silent --show-error --fail --max-time 3 \
    -H "Authorization: Bearer $token" -H 'Accept: application/json' \
    "$api" > "$scratch/overview.json"
python3 - "$scratch/overview.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as src:
    result = json.load(src)
assert result.get("api_version") == "v1", result
assert result.get("state") == "connected", result
assert result.get("engine") == "Caddy-compatible", result
assert isinstance(result.get("servers"), int) and result["servers"] >= 1
assert isinstance(result.get("routes"), int) and result["routes"] >= 1
assert isinstance(result.get("hosts"), list)
assert isinstance(result.get("host_count"), int)
assert set(result).issubset({
    "api_version", "engine", "state", "servers", "routes",
    "hosts", "host_count", "checked_at"
}), result
PY

test "$(http_code -H "Authorization: Bearer $token" \
    -H 'Sec-Fetch-Site: cross-site' "$api")" = 403

curl --silent --show-error --fail --max-time 3 \
    -D "$scratch/headers" -o "$scratch/app" \
    http://127.0.0.1:7689/
grep -iq '^Content-Security-Policy:' "$scratch/headers"
grep -iq '^X-Frame-Options: DENY' "$scratch/headers"
grep -iq '^Cache-Control: no-store' "$scratch/headers"
grep -q 'GoreeCloud Gateway' "$scratch/app"
echo "PASS: connected Gateway+console, token throttle, reduced JSON and web headers"
