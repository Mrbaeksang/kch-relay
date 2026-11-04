#!/usr/bin/env bash
set -euo pipefail
image="${1:-kch-relay:local}"
name="relay-smoke-${RANDOM}"
volume="${name}-data"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT
docker volume create "$volume" >/dev/null
start() {
  docker run -d --name "$name" -p 127.0.0.1::8080 -v "$volume:/data" "$image" >/dev/null
  address="http://$(docker port "$name" 8080/tcp)"
  for attempt in $(seq 1 40); do
    if curl -fsS "$address/api/readyz" >/dev/null; then return; fi
    sleep 1
  done
  docker logs "$name"
  exit 1
}
start
curl -fsS "$address/" | grep -q 'KCH Relay'
status="$(curl -s -o /dev/null -w '%{http_code}' "$address/api/v1/products")"
test "$status" = '404'
docker exec "$name" sh -c 'test -s /data/market.db && printf "retained" > /data/uploads/smoke.txt'
docker rm -f "$name" >/dev/null
start
test "$(docker exec "$name" cat /data/uploads/smoke.txt)" = 'retained'
echo 'Static frontend, API readiness, unknown API, and volume persistence: OK'
