#!/usr/bin/env bash

set -euo pipefail

P42_CTL=${P42_CTL:-p42-ctl}
read -r -a P42_ARGS_ARRAY <<<"${P42_ARGS:-}"

log() {
	printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

err() {
	log "$*" >&2
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

TENANT_FILTER=()

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
			cat <<'EOF'
Usage: migrate_github_creds.sh [--tenant-id TENANT_ID]

Run the one-time migration that copies legacy tenant GitHub credentials
into GitHub connections and sets the tenant default connection.

Environment variables:
  P42_CTL   - Path to the CLI binary (default: p42-ctl)
  P42_ARGS  - Extra arguments for every CLI invocation (e.g. "--dev")

Requires: jq
EOF
			exit 0
			;;
		*)
			err "Unknown argument: $1"
			exit 1
			;;
	esac
done

require_cmd jq
require_cmd "$P42_CTL"

lower() {
	printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

json_escape() {
	jq -rn --arg value "$1" '$value'
}

collect_tenants() {
	if ((${#TENANT_FILTER[@]} > 0)); then
		printf '%s\n' "${TENANT_FILTER[@]}"
		return
	fi
	mapfile -t tenants < <(run_ctl tenant list | jq -r 'select(.Deleted != true) | .TenantId')
	printf '%s\n' "${tenants[@]}"
}

fetch_json() {
	local description=$1
	shift
	local err_file
	err_file=$(mktemp)
	if ! output=$(run_ctl "$@" 2>"$err_file"); then
		if grep -q '404' "$err_file"; then
			rm -f "$err_file"
			return 104
		fi
		err "Failed to $description"
		cat "$err_file" >&2
		rm -f "$err_file"
		return 1
	fi
	rm -f "$err_file"
	printf '%s' "$output" | jq -c '.'
}

ensure_connection() {
	local tenant_id=$1
	local login_lower=$2
	local github_login=$3
	local github_user_id=$4

	local existing
	existing=$(run_ctl github list-connections -i "$tenant_id" | jq -c \
		--arg login "$login_lower" --argjson uid "$github_user_id" \
		'select(.Private == false) | select((.GithubUserID != null and .GithubUserID == $uid) or (.GithubUserLogin != null and (ascii_downcase(.GithubUserLogin) == $login)))' | head -n 1 || true)
	if [[ -n "$existing" ]]; then
		printf '%s' "$existing"
		return 0
	fi

	local payload
	payload=$(jq -n --arg login "$github_login" --argjson uid "$github_user_id" '{Private:false, GithubUserLogin:$login, GithubUserID:$uid}')
	run_ctl github add-connection -i "$tenant_id" <<-EOF | jq -c '.'
	$payload
EOF
}

update_connection() {
	local tenant_id=$1
	local connection_id=$2
	local github_login=$3
	local github_user_id=$4
	local oauth_token=$5
	local refresh_token=$6

	local payload
	payload=$(jq -n --arg login "$github_login" --argjson uid "$github_user_id" \
		--arg oauth "$oauth_token" --arg refresh "$refresh_token" \
		'{GithubUserLogin:$login, GithubUserID:$uid} + (if $oauth != "" then {OAuthToken:$oauth} else {} end) + (if $refresh != "" then {RefreshToken:$refresh} else {} end)')
	run_ctl github update-connection -i "$tenant_id" -c "$connection_id" <<-EOF >/dev/null
	$payload
EOF
}

set_default_connection() {
	local tenant_id=$1
	local connection_id=$2
	run_ctl tenant update -i "$tenant_id" <<-EOF >/dev/null
{
  "DefaultGithubConnectionID": "${connection_id}"
}
EOF
}

migrate_tenant() {
	local tenant_id=$1
	log "Tenant ${tenant_id}: starting"

	local tenant_json
	if ! tenant_json=$(fetch_json "fetch tenant" tenant get -i "$tenant_id"); then
		return 1
	fi

	local default_connection
	default_connection=$(jq -r '.DefaultGithubConnectionID // ""' <<<"$tenant_json")

	local creds_json
	creds_json=$(fetch_json "fetch github creds" github get-tenant-creds -i "$tenant_id") || {
		local status=$?
		if [[ $status -eq 104 ]]; then
			log "Tenant ${tenant_id}: no GitHub creds, skipping"
			return 0
		fi
		return 1
	}

	local github_login
	github_login=$(jq -r '.GithubUserLogin // ""' <<<"$creds_json")
	local github_user_id
	github_user_id=$(jq '.GithubUserID' <<<"$creds_json")
	local oauth_token
	oauth_token=$(jq -r '.OAuthToken // ""' <<<"$creds_json")
	local refresh_token
	refresh_token=$(jq -r '.RefreshToken // ""' <<<"$creds_json")

	if [[ -z "$github_login" || "$github_user_id" == "null" ]]; then
		log "Tenant ${tenant_id}: missing GitHub login or user ID, skipping"
		return 0
	fi
	if [[ -z "$oauth_token" && -z "$refresh_token" ]]; then
		log "Tenant ${tenant_id}: no tokens to migrate, skipping"
		return 0
	fi

	local connection_json
	connection_json=$(ensure_connection "$tenant_id" "$(lower "$github_login")" "$github_login" "$github_user_id") || {
		err "Tenant ${tenant_id}: failed to ensure connection"
		return 1
	}

	local connection_id
	connection_id=$(jq -r '.ConnectionID' <<<"$connection_json")
	log "Tenant ${tenant_id}: using connection ${connection_id}"

	update_connection "$tenant_id" "$connection_id" "$github_login" "$github_user_id" "$oauth_token" "$refresh_token"
	log "Tenant ${tenant_id}: updated connection tokens"

	if [[ "$default_connection" != "$connection_id" ]]; then
		set_default_connection "$tenant_id" "$connection_id"
		log "Tenant ${tenant_id}: set default connection"
	else
		log "Tenant ${tenant_id}: default connection already set"
	fi

	log "Tenant ${tenant_id}: finished"
}

main() {
	local overall_rc=0
	while IFS= read -r tenant_id; do
		[[ -z "$tenant_id" ]] && continue
		if ! migrate_tenant "$tenant_id"; then
			overall_rc=1
			err "Tenant ${tenant_id}: migration failed"
		fi
	done < <(collect_tenants)
	return $overall_rc
}

main "$@"
