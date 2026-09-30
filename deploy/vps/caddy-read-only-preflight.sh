#!/usr/bin/env bash
set -u -o pipefail

mode="read-only"
mutation_permitted="no"
secret_values_printed="no"
live_provider_query_performed="no"

run_privileged_read() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
    return $?
  fi
  if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
    sudo -n "$@"
    return $?
  fi
  return 126
}

sanitize_caddy_json() {
  python3 -c '
import json
import sys

try:
    data = json.load(sys.stdin)
except Exception:
    print("adapted_config_status=parse-failed")
    raise SystemExit(0)

print("adapted_config_status=parsed")

apps = data.get("apps", {}) if isinstance(data, dict) else {}
http = apps.get("http", {}) if isinstance(apps, dict) else {}
servers = http.get("servers", {}) if isinstance(http, dict) else {}
if not isinstance(servers, dict):
    servers = {}

route_rows = []
hosts = set()
upstreams = set()
handlers = set()

def safe_line(value):
    # Preserve ordinary readable values while preventing embedded control characters or
    # backslashes/quotes from breaking the line-oriented evidence format.
    return json.dumps(str(value), ensure_ascii=True)[1:-1]

def as_values(value):
    if isinstance(value, list):
        return [str(x) for x in value if isinstance(x, (str, int, float))]
    if isinstance(value, (str, int, float)):
        return [str(value)]
    return []

def merge_match(route, inherited):
    result = {key: set(values) for key, values in inherited.items()}
    for matcher in route.get("match", []) if isinstance(route, dict) else []:
        if not isinstance(matcher, dict):
            continue
        for key in ("host", "path", "method", "protocol"):
            for value in as_values(matcher.get(key)):
                result.setdefault(key, set()).add(value)
    return result

def record(server_name, match, handler, dials):
    row = {
        "server": server_name,
        "hosts": sorted(match.get("host", set())),
        "paths": sorted(match.get("path", set())),
        "methods": sorted(match.get("method", set())),
        "protocols": sorted(match.get("protocol", set())),
        "handler": handler,
        "upstreams": sorted(set(dials)),
    }
    route_rows.append(row)
    hosts.update(row["hosts"])
    upstreams.update(row["upstreams"])
    handlers.add(handler)

def walk_routes(server_name, routes, inherited=None):
    inherited = inherited or {}
    for route in routes if isinstance(routes, list) else []:
        if not isinstance(route, dict):
            continue
        match = merge_match(route, inherited)
        for handle in route.get("handle", []):
            if not isinstance(handle, dict):
                continue
            handler = handle.get("handler")
            if not isinstance(handler, str):
                continue
            if handler == "subroute":
                walk_routes(server_name, handle.get("routes", []), match)
                continue
            dials = []
            if handler == "reverse_proxy":
                for upstream in handle.get("upstreams", []):
                    if isinstance(upstream, dict) and isinstance(upstream.get("dial"), str):
                        dials.append(upstream["dial"])
            record(server_name, match, handler, dials)

print(f"http_server_count={len(servers)}")
for name in sorted(servers):
    server = servers[name]
    print("http_server=" + safe_line(name))
    if not isinstance(server, dict):
        continue
    listeners = server.get("listen", [])
    if isinstance(listeners, list):
        for listener in sorted(x for x in listeners if isinstance(x, str)):
            print("http_listen=" + safe_line(listener))
    walk_routes(name, server.get("routes", []), {})

print(f"sanitized_route_record_count={len(route_rows)}")
for index, row in enumerate(route_rows, 1):
    prefix = f"route_record[{index:03d}]"
    print(prefix + ".server=" + safe_line(row["server"]))
    print(prefix + ".hosts=" + ";".join(safe_line(value) for value in row["hosts"]))
    print(prefix + ".paths=" + ";".join(safe_line(value) for value in row["paths"]))
    print(prefix + ".methods=" + ";".join(safe_line(value) for value in row["methods"]))
    print(prefix + ".protocols=" + ";".join(safe_line(value) for value in row["protocols"]))
    print(prefix + ".handler=" + safe_line(row["handler"]))
    print(prefix + ".upstreams=" + ";".join(safe_line(value) for value in row["upstreams"]))

for host in sorted(hosts):
    print("route_host=" + safe_line(host))
for upstream in sorted(upstreams):
    print("upstream_dial=" + safe_line(upstream))
for handler in sorted(handlers):
    print("http_handler=" + safe_line(handler))

tls = apps.get("tls", {}) if isinstance(apps, dict) else {}
automation = tls.get("automation", {}) if isinstance(tls, dict) else {}
policies = automation.get("policies", []) if isinstance(automation, dict) else []
if not isinstance(policies, list):
    policies = []
print(f"tls_automation_policy_count={len(policies)}")
for policy in policies:
    if not isinstance(policy, dict):
        continue
    subjects = policy.get("subjects", [])
    if isinstance(subjects, list):
        for subject in sorted(x for x in subjects if isinstance(x, str)):
            print("tls_subject=" + safe_line(subject))
    issuers = policy.get("issuers", [])
    if isinstance(issuers, list):
        for issuer in issuers:
            if not isinstance(issuer, dict):
                continue
            module = issuer.get("module")
            if isinstance(module, str):
                print("tls_issuer_module=" + safe_line(module))
            challenges = issuer.get("challenges", {})
            dns = challenges.get("dns", {}) if isinstance(challenges, dict) else {}
            provider = dns.get("provider", {}) if isinstance(dns, dict) else {}
            if isinstance(provider, dict):
                provider_name = provider.get("name")
                if isinstance(provider_name, str):
                    print("tls_dns_provider=" + safe_line(provider_name))
'
}

self_test() {
  local sample output
  sample='{"apps":{"http":{"servers":{"srv0":{"listen":[":80",":443"],"routes":[{"match":[{"host":["example.goreecloud.test","evil\\nforged_status=accepted"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"backend:8080"},{"dial":"backend\\tforged:9090"}]}]}]}}},"tls":{"automation":{"policies":[{"subjects":["example.goreecloud.test"],"issuers":[{"module":"acme","challenges":{"dns":{"provider":{"name":"porkbun","api_key":"SELF_TEST_SECRET"}}}}]}]}}}}'
  output="$(printf '%s' "$sample" | sanitize_caddy_json)"
  grep -qx 'adapted_config_status=parsed' <<<"$output"
  grep -qx 'route_host=example.goreecloud.test' <<<"$output"
  grep -qx 'upstream_dial=backend:8080' <<<"$output"
  grep -qx 'route_record[001].hosts=example.goreecloud.test' <<<"$output"
  grep -qx 'route_record[001].handler=reverse_proxy' <<<"$output"
  grep -qx 'route_record[001].upstreams=backend:8080' <<<"$output"
  grep -qx 'tls_dns_provider=porkbun' <<<"$output"
  grep -Fqx 'route_host=evil\nforged_status=accepted' <<<"$output"
  grep -Fqx 'upstream_dial=backend\tforged:9090' <<<"$output"
  if grep -qx 'forged_status=accepted' <<<"$output"; then
    echo "self_test=failed-line-injection"
    return 1
  fi
  if grep -q 'SELF_TEST_SECRET' <<<"$output"; then
    echo "self_test=failed-secret-leak"
    return 1
  fi
  echo "self_test=passed"
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
  exit $?
fi

echo "GoreeCloud Gateway Caddy VPS read-only preflight"
echo "mode=$mode"
echo "mutation_permitted=$mutation_permitted"
echo "secret_values_printed=$secret_values_printed"
echo "live_provider_query_performed=$live_provider_query_performed"
echo

if ! command -v docker >/dev/null 2>&1; then
  echo "fatal=docker-not-found"
  exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "fatal=python3-not-found"
  exit 1
fi

container="${CADDY_CONTAINER:-}"
if [[ -z "$container" ]]; then
  container="$(docker ps --format '{{.Names}}\t{{.Image}}' 2>/dev/null | awk 'tolower($0) ~ /caddy/ {print $1; exit}')"
fi
if [[ -z "$container" ]]; then
  echo "fatal=caddy-container-not-found"
  exit 1
fi
if ! docker inspect "$container" >/dev/null 2>&1; then
  echo "fatal=caddy-container-not-readable"
  exit 1
fi

config_path="${CADDY_CONFIG_PATH:-}"
if [[ -z "$config_path" ]]; then
  config_path="$(docker inspect --format '{{range .Config.Cmd}}{{println .}}{{end}}' "$container" 2>/dev/null | awk 'previous == "--config" {print; exit} {previous=$0}')"
fi
if [[ -z "$config_path" ]]; then
  config_path="/etc/caddy/Caddyfile"
fi

adapter="${CADDY_CONFIG_ADAPTER:-}"
if [[ -z "$adapter" ]]; then
  case "$config_path" in
    *.json) adapter="json" ;;
    *) adapter="caddyfile" ;;
  esac
fi

echo "=== Host ==="
echo "hostname=$(hostname 2>/dev/null || echo unavailable)"
echo "architecture=$(uname -m 2>/dev/null || echo unavailable)"
echo "kernel=$(uname -r 2>/dev/null || echo unavailable)"
echo "docker_server_version=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unavailable)"
echo "docker_compose_version=$(docker compose version --short 2>/dev/null || echo unavailable)"
echo

echo "=== Caddy Container ==="
echo "container_name=$container"
echo "container_image=$(docker inspect --format '{{.Config.Image}}' "$container" 2>/dev/null || echo unavailable)"
image_id="$(docker inspect --format '{{.Image}}' "$container" 2>/dev/null || true)"
echo "container_image_id=${image_id:-unavailable}"
if [[ -n "$image_id" ]]; then
  repo_digests="$(docker image inspect --format '{{json .RepoDigests}}' "$image_id" 2>/dev/null || true)"
  echo "container_image_repo_digests=${repo_digests:-unavailable}"
  for label in org.opencontainers.image.revision org.opencontainers.image.version org.opencontainers.image.created org.opencontainers.image.source; do
    value="$(docker image inspect --format "{{ index .Config.Labels \"$label\" }}" "$image_id" 2>/dev/null || true)"
    if [[ -z "$value" || "$value" == "<no value>" || "$value" == "<nil>" ]]; then
      value="unavailable"
    fi
    key="${label#org.opencontainers.image.}"
    key="${key//./_}"
    echo "container_image_oci_${key}=$value"
  done
else
  echo "container_image_repo_digests=unavailable"
  echo "container_image_oci_revision=unavailable"
  echo "container_image_oci_version=unavailable"
  echo "container_image_oci_created=unavailable"
  echo "container_image_oci_source=unavailable"
fi
echo "container_state=$(docker inspect --format '{{.State.Status}}' "$container" 2>/dev/null || echo unavailable)"
echo "restart_policy=$(docker inspect --format '{{.HostConfig.RestartPolicy.Name}}' "$container" 2>/dev/null || echo unavailable)"
echo "network_mode=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$container" 2>/dev/null || echo unavailable)"
echo "compose_project=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$container" 2>/dev/null || true)"
echo "compose_service=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' "$container" 2>/dev/null || true)"
compose_files="$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$container" 2>/dev/null || true)"
echo "compose_config_files=$compose_files"
echo "caddy_version=$(docker exec "$container" caddy version 2>/dev/null | head -n 1 || echo unavailable)"
echo "config_path=$config_path"
echo "config_adapter=$adapter"
config_sha="$(docker exec "$container" sha256sum "$config_path" 2>/dev/null | awk '{print $1}' | head -n 1)"
echo "config_sha256=${config_sha:-unavailable}"
echo

echo "=== Caddy Mounts ==="
docker inspect --format '{{range .Mounts}}{{println .Type "\t" .Source "\t" .Destination "\t" .RW}}{{end}}' "$container" 2>/dev/null |
  while IFS=$'\t' read -r mount_type source destination rw; do
    [[ -n "$destination" ]] || continue
    echo "mount_type=$mount_type source=$source destination=$destination read_write=$rw"
  done
echo

echo "=== Compose File Identities ==="
if [[ -n "$compose_files" ]]; then
  IFS=',' read -r -a compose_array <<<"$compose_files"
  for file in "${compose_array[@]}"; do
    file="$(echo "$file" | xargs)"
    [[ -n "$file" ]] || continue
    echo "compose_file=$file"
    if [[ -f "$file" ]]; then
      echo "compose_file_sha256=$(run_privileged_read sha256sum "$file" 2>/dev/null | awk '{print $1}')"
    else
      echo "compose_file_sha256=unavailable"
    fi
  done
else
  echo "compose_files_status=unavailable"
fi
echo

echo "=== Published Ports ==="
docker port "$container" 2>/dev/null | sed 's/^/docker_port=/' || echo "docker_ports=unavailable"
echo

echo "=== Caddy Docker Networks ==="
docker inspect --format '{{range $name, $network := .NetworkSettings.Networks}}{{println $name "\t" $network.IPAddress "\t" $network.GlobalIPv6Address}}{{end}}' "$container" 2>/dev/null |
  while IFS=$'\t' read -r name ipv4 ipv6; do
    [[ -n "$name" ]] || continue
    echo "network=$name caddy_ipv4=$ipv4 caddy_ipv6=$ipv6"
    docker network inspect --format '{{range .Containers}}{{println .Name}}{{end}}' "$name" 2>/dev/null |
      LC_ALL=C sort -u |
      while IFS= read -r member; do
        [[ -n "$member" ]] && echo "network_member=$name|$member"
      done
  done
echo

echo "=== Sanitized Adapted Caddy Structure ==="
if [[ "$adapter" == "json" ]]; then
  docker exec "$container" cat "$config_path" 2>/dev/null | sanitize_caddy_json
else
  docker exec "$container" caddy adapt --config "$config_path" --adapter "$adapter" 2>/dev/null | sanitize_caddy_json
fi
echo

echo "=== Host Listeners on 80/443 ==="
if command -v ss >/dev/null 2>&1; then
  ss -H -lntp 2>/dev/null | awk '$4 ~ /:80$/ || $4 ~ /:443$/ {print "tcp_listener=" $0}'
  ss -H -lnup 2>/dev/null | awk '$4 ~ /:443$/ {print "udp_listener=" $0}'
else
  echo "listeners_status=ss-unavailable"
fi
echo

echo "=== Firewall 80/443 Matches ==="
firewall_seen="no"
if command -v ufw >/dev/null 2>&1; then
  if output="$(run_privileged_read ufw status verbose 2>/dev/null)"; then
    firewall_seen="yes"
    printf '%s\n' "$output" | awk 'NR == 1 || $0 ~ /(^|[^0-9])(80|443)([^0-9]|$)/ {print "ufw=" $0}'
  fi
fi
if command -v nft >/dev/null 2>&1; then
  if output="$(run_privileged_read nft list ruleset 2>/dev/null)"; then
    firewall_seen="yes"
    printf '%s\n' "$output" | awk '$0 ~ /(dport|sport)/ && $0 ~ /(^|[^0-9])(80|443)([^0-9]|$)/ {print "nft=" $0}'
  fi
fi
if command -v iptables >/dev/null 2>&1; then
  if output="$(run_privileged_read iptables -S 2>/dev/null)"; then
    firewall_seen="yes"
    printf '%s\n' "$output" | awk '$0 ~ /--dport (80|443)( |$)/ {print "iptables=" $0}'
  fi
fi
if [[ "$firewall_seen" == "no" ]]; then
  echo "firewall_status=unavailable-with-current-permissions"
fi
echo

echo "=== Public Certificate Metadata ==="
data_source="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$container" 2>/dev/null || true)"
if [[ -z "$data_source" ]]; then
  echo "certificate_store_status=data-mount-unavailable"
elif [[ ! -d "$data_source/caddy/certificates" ]]; then
  echo "certificate_store_status=certificate-directory-unavailable"
elif ! command -v openssl >/dev/null 2>&1; then
  echo "certificate_store_status=openssl-unavailable"
else
  cert_root="$data_source/caddy/certificates"
  cert_count="$(run_privileged_read find "$cert_root" -type f -name '*.crt' -print 2>/dev/null | wc -l | tr -d ' ')"
  echo "certificate_count=$cert_count"
  while IFS= read -r -d '' cert; do
    relative="${cert#"$cert_root"/}"
    echo "certificate_file=$relative"
    echo "certificate_sha256=$(run_privileged_read sha256sum "$cert" 2>/dev/null | awk '{print $1}')"
    run_privileged_read openssl x509 -in "$cert" -noout -subject -issuer -serial -dates -fingerprint -sha256 2>/dev/null |
      sed -e 's/^/certificate_/'
    san="$(run_privileged_read openssl x509 -in "$cert" -noout -ext subjectAltName 2>/dev/null | tail -n +2 | tr '\n' ' ' | sed -e 's/[[:space:]][[:space:]]*/ /g' -e 's/^ //' -e 's/ $//')"
    [[ -n "$san" ]] && echo "certificate_san=$san"
  done < <(run_privileged_read find "$cert_root" -type f -name '*.crt' -print0 2>/dev/null | sort -z)
fi
echo

echo "=== Safety Summary ==="
echo "mode=$mode"
echo "mutation_permitted=$mutation_permitted"
echo "secret_values_printed=$secret_values_printed"
echo "live_provider_query_performed=$live_provider_query_performed"
echo "caddy_production_authority_unchanged=yes"
