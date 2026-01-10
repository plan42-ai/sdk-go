# sdk-go

The go sdk for the plan42 api.

## Utility Scripts

The `scripts/migrate_github_creds.sh` helper performs the one-time migration of
legacy tenant GitHub credentials into GitHub connections using the existing
`p42-ctl` CLI. The script accepts optional `--tenant-id` filters and honors the
`P42_CTL`/`P42_ARGS` environment variables so it can target different API
endpoints (for example, `P42_ARGS="--dev"`).
