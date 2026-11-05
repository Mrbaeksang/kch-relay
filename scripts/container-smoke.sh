#!/usr/bin/env bash
set -euo pipefail
image="${1:-kch-relay:local}"
name="relay-smoke-${RANDOM}"
volume="${name}-data"
workspace="$(mktemp -d)"
origin="http://contract.local"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -rf "$workspace"
}
trap cleanup EXIT
docker volume create "$volume" >/dev/null
start() {
  mode="$1"
  if [ "$mode" = development ]; then
    docker run -d --name "$name" -p 127.0.0.1::8080 -v "$volume:/data" -e APP_ENV=development -e AUTH_MODE=development -e APP_ORIGIN="$origin" "$image" >/dev/null
  else
    docker run -d --name "$name" -p 127.0.0.1::8080 -v "$volume:/data" "$image" >/dev/null
  fi
  address="http://$(docker port "$name" 8080/tcp)"
  for attempt in $(seq 1 40); do
    if curl -fsS "$address/api/readyz" >/dev/null 2>&1; then return; fi
    sleep 1
  done
  docker logs "$name"
  exit 1
}
start development
curl -fsS "$address/" | grep -q 'KCH Relay'
python3 scripts/verify_contract.py --base-url "$address" --origin "$origin" --mode development
python3 scripts/verify_contract.py --base-url "$address" --origin "$origin" --mode development --cookie-file "$workspace/cookies" --session-probe create
docker exec "$name" sh -c 'test -s /data/market.db && printf "retained" > /data/uploads/smoke.txt'
docker stop --time 15 "$name" >/dev/null
docker rm "$name" >/dev/null
start development
test "$(docker exec "$name" cat /data/uploads/smoke.txt)" = retained
python3 scripts/verify_contract.py --base-url "$address" --origin "$origin" --mode development --cookie-file "$workspace/cookies" --session-probe resume
docker rm -f "$name" >/dev/null
start disabled
python3 scripts/verify_contract.py --base-url "$address" --mode disabled
if docker run --rm -e APP_ENV=production -e AUTH_MODE=development "$image" >"$workspace/invalid-mode.log" 2>&1; then
  echo 'Invalid production/development combination unexpectedly started'
  exit 1
fi
echo 'Actual container: frontend, auth, role/CSRF, disabled mode, DB/file/session persistence OK'
