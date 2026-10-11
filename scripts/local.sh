#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
project=${LOCAL_PROJECT:-smallchop-local}
case "$project" in
    smallchop-local|smallchop-local-*) ;;
    *) echo 'LOCAL_PROJECT must be smallchop-local or start with smallchop-local-' >&2; exit 1 ;;
esac

compose() {
    docker compose --project-directory . --env-file .env.local --project-name "$project" -f deploy/compose.local.yml "$@"
}

if [ ! -f .env.local ]; then
    echo 'First run: cp deploy/env/local.env.example .env.local' >&2
    exit 1
fi

case "${1:-}" in
    up) compose up --build -d --wait --wait-timeout 120 ;;
    status) compose ps ;;
    metrics) compose exec -T app wget -q -O - http://127.0.0.1:9090/metrics ;;
    logs) compose logs --tail 100 ;;
    stop) compose stop ;;
    down) compose down ;;
    reset)
        if [ "${2:-}" != '--delete-local-data' ]; then
            echo 'Reset deletes this local project’s MongoDB data. Use: scripts/local.sh reset --delete-local-data' >&2
            exit 1
        fi
        compose down --volumes
        ;;
    smoke)
        endpoint=$(compose port caddy 8080)
        python3 scripts/smoke-local.py "http://$endpoint"
        ;;
    *) echo 'Usage: scripts/local.sh {up|status|metrics|logs|stop|down|reset --delete-local-data|smoke}' >&2; exit 1 ;;
esac
