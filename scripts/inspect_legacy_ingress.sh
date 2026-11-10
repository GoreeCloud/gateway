#!/usr/bin/env bash
# Read-only legacy Docker ingress inspection; no credentials, config dumps, or changes.
set -euo pipefail

if (( $# > 0 )); then
  name="$1"
else
  name="caddy"
fi
if [[ ! "$name" =~ ^[[:alnum:]][[:alnum:]_.-]{0,62}$ ]]; then
  printf 'error=invalid_container_name\n' >&2
  exit 2
fi
if ! command -v docker >/dev/null 2>&1; then
  printf 'error=docker_unavailable\n' >&2
  exit 2
fi
if ! docker inspect --type container "$name" >/dev/null 2>&1; then
  printf 'container=not_found\n'
  exit 2
fi

read_field() {
  docker inspect --type container --format "$1" "$name" 2>/dev/null
}

status="$(read_field '{{.State.Status}}')"
exit_code="$(read_field '{{.State.ExitCode}}')"
restart_count="$(read_field '{{.RestartCount}}')"
health_state="$(read_field '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}')"
[[ "$restart_count" =~ ^[0-9]{1,10}$ ]] || restart_count=unknown
case "$health_state" in
  healthy|unhealthy|starting|none) ;;
  *) health_state=unknown ;;
esac
restart="$(read_field '{{.HostConfig.RestartPolicy.Name}}')"
error_text="$(read_field '{{.State.Error}}')"
case "$error_text" in
  *'Address already in use'*) error_kind='address_conflict' ;;
  '') error_kind='none' ;;
  *) error_kind='other' ;;
esac
mount_count="$(read_field '{{len .Mounts}}')"
binding_count="$(read_field '{{len .HostConfig.PortBindings}}')"
network_mode="$(read_field '{{.HostConfig.NetworkMode}}')"

printf 'inspection=read_only\n'
printf 'container_present=true\n'
printf 'container_status=%s\n' "$status"
printf 'restart_count=%s\n' "$restart_count"
printf 'docker_health_state=%s\n' "$health_state"
printf 'exit_code=%s\n' "$exit_code"
printf 'error_category=%s\n' "$error_kind"
printf 'restart_policy=%s\n' "$restart"
printf 'volume_mount_count=%s\n' "$mount_count"
printf 'published_port_binding_count=%s\n' "$binding_count"
if [[ -z "$network_mode" ]]; then
  printf 'network_mode_set=false\n'
else
  printf 'network_mode_set=true\n'
fi
if command -v ss >/dev/null 2>&1; then
  if ss -H -lnut '( sport = :80 or sport = :443 )' >/dev/null 2>&1; then
    listeners="$(ss -H -lnut '( sport = :80 or sport = :443 )' | wc -l)"
    printf 'web_listener_count=%s\n' "$listeners"
  else
    printf 'web_listener_count=unavailable\n'
  fi
else
  printf 'web_listener_count=unavailable\n'
fi
printf 'recovery_approval=required\n'
printf 'automated_restart=never\n'
# Nonzero means running state/declared health was not confirmed, NOT a root cause.
# No configured healthcheck (none) remains unverified, not "healthy".
if [[ "$status" != 'running' || "$health_state" == 'unhealthy' ||
      "$health_state" == 'starting' || "$health_state" == 'unknown' ]]; then
  exit 1
fi
