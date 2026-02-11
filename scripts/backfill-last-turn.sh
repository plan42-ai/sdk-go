#!/usr/bin/env bash

set -euo pipefail

P42_CTL=${P42_CTL:-p42-ctl}
P42_ARGS_ARRAY=()
if [[ -n ${P42_ARGS:-} ]]; then
  read -r -a P42_ARGS_ARRAY <<<"${P42_ARGS}"
fi

TENANT_FILTER=()
TENANT_IDS=()

log() {
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2
}

err() {
  log "ERROR: $*"
}

usage() {
  cat <<'EOF'
Usage: backfill-last-turn.sh [options]

Backfills LastTurnStatus/LastTurnIndex by issuing empty turn updates on the
last turn for every task in each tenant.

Options:
  -t, --tenant-id ID   Limit processing to the specified tenant. Repeatable.
  -h, --help           Show this help message and exit.

Environment variables:
  P42_CTL   Path to the p42-ctl binary (default: p42-ctl)
  P42_ARGS  Extra arguments to pass to every p42-ctl invocation

Requires: jq
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    err "Missing required command: $1"
    exit 1
  fi
}

run_ctl() {
  "$P42_CTL" "${P42_ARGS_ARRAY[@]}" "$@"
}

fetch_last_turn() {
  local tenant_id=$1
  local task_id=$2
  local workstream_id=$3
  local args=(turn get-last -i "$tenant_id" -t "$task_id" -d)
  if [[ -n "$workstream_id" ]]; then
    args+=(-w "$workstream_id")
  fi
  run_ctl "${args[@]}" | jq -c '.'
}

update_turn() {
  local tenant_id=$1
  local task_id=$2
  local turn_index=$3
  local workstream_id=$4
  local args=(turn update -i "$tenant_id" -t "$task_id" -n "$turn_index")
  if [[ -n "$workstream_id" ]]; then
    args+=(-w "$workstream_id")
  fi
  run_ctl "${args[@]}" >/dev/null <<<'{}'
}

process_task() {
  local tenant_id=$1
  local task_json=$2
  local task_id
  task_id=$(jq -r '.TaskId' <<<"$task_json")
  if [[ -z "$task_id" || "$task_id" == "null" ]]; then
    err "Tenant ${tenant_id}: encountered task with no TaskId"
    return 1
  fi
  local workstream_id
  workstream_id=$(jq -r '.WorkstreamId // ""' <<<"$task_json")
  local deleted
  deleted=$(jq -r '.Deleted // false' <<<"$task_json")
  local label="Tenant ${tenant_id}: task ${task_id}"
  if [[ -n "$workstream_id" ]]; then
    label+=" (workstream ${workstream_id})"
  fi
  if [[ "$deleted" == "true" ]]; then
    label+=" [deleted]"
  fi
  log "${label}: fetching last turn"
  local last_turn_json
  if ! last_turn_json=$(fetch_last_turn "$tenant_id" "$task_id" "$workstream_id"); then
    err "${label}: failed to fetch last turn"
    return 1
  fi
  local turn_index
  turn_index=$(jq -r '.TurnIndex' <<<"$last_turn_json")
  if [[ -z "$turn_index" || "$turn_index" == "null" ]]; then
    log "${label}: no last turn index returned, skipping"
    return 2
  fi
  if ! [[ "$turn_index" =~ ^-?[0-9]+$ ]]; then
    err "${label}: invalid turn index '${turn_index}'"
    return 1
  fi
  if ! update_turn "$tenant_id" "$task_id" "$turn_index" "$workstream_id"; then
    err "${label}: turn update failed"
    return 1
  fi
  log "${label}: backfill applied to turn ${turn_index}"
  return 0
}

process_tenant() {
  local tenant_id=$1
  log "Tenant ${tenant_id}: listing tasks"
  local tasks=()
  if ! mapfile -t tasks < <(run_ctl task list -i "$tenant_id" -d | jq -c '.'); then
    err "Tenant ${tenant_id}: failed to list tasks"
    return 1
  fi
  if ((${#tasks[@]} == 0)); then
    log "Tenant ${tenant_id}: no tasks found"
    return 0
  fi
  local updated=0
  local skipped=0
  local failed=0
  for task_json in "${tasks[@]}"; do
    [[ -z "$task_json" ]] && continue
    if process_task "$tenant_id" "$task_json"; then
      ((updated++))
    else
      local rc=$?
      if [[ $rc -eq 2 ]]; then
        ((skipped++))
      else
        ((failed++))
      fi
    fi
  done
  log "Tenant ${tenant_id}: completed (updated=${updated}, skipped=${skipped}, failed=${failed})"
  if ((failed > 0)); then
    return 1
  fi
  return 0
}

populate_tenant_ids() {
  if ((${#TENANT_FILTER[@]} > 0)); then
    TENANT_IDS=()
    for tenant_id in "${TENANT_FILTER[@]}"; do
      [[ -z "$tenant_id" ]] && continue
      TENANT_IDS+=("$tenant_id")
    done
    log "Queued ${#TENANT_IDS[@]} tenant(s) for targeted backfill"
    return 0
  fi
  log "Fetching active tenants via ${P42_CTL}"
  local discovered=()
  if ! mapfile -t discovered < <(run_ctl tenant list | jq -r 'select(.Deleted != true) | .TenantId'); then
    err "Failed to discover tenants"
    return 1
  fi
  TENANT_IDS=()
  for tenant_id in "${discovered[@]}"; do
    [[ -z "$tenant_id" ]] && continue
    TENANT_IDS+=("$tenant_id")
  done
  log "Discovered ${#TENANT_IDS[@]} active tenant(s)"
  return 0
}

main() {
  if ! populate_tenant_ids; then
    return 1
  fi
  if ((${#TENANT_IDS[@]} == 0)); then
    log "No tenants to process"
    return 0
  fi
  local overall_rc=0
  for tenant_id in "${TENANT_IDS[@]}"; do
    if ! process_tenant "$tenant_id"; then
      overall_rc=1
    fi
  done
  return $overall_rc
}

while (($#)); do
  case "$1" in
    -t|--tenant-id)
      if (($# < 2)); then
        err "--tenant-id requires a value"
        exit 1
      fi
      TENANT_FILTER+=("$2")
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    *)
      err "Unknown argument: $1"
      usage >&2
      exit 1
      ;;
  esac
done

if (($# > 0)); then
  err "Unexpected arguments: $*"
  usage >&2
  exit 1
fi

require_cmd jq
require_cmd "$P42_CTL"

if main; then
  exit 0
fi
exit 1
