#!/usr/bin/env bash
# Local JetStream resources for continuous ingestion. Requires nats-server + nats.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: bash scripts/nats-fixture.sh [--port 14223] [--count 20] [--interval 0]

Starts a fresh local JetStream server and seeds JSON messages on:
  orders.>       Two physical streams: orders.us.> and orders.eu.>
  products.>     Created and updated events in one stream
  inventory.>    Stock adjustments
  customers.>    Customer events

--count       Initial messages per concrete subject (6 subjects).
--interval    Seconds between additional rounds; 0 seeds once and stays running.

Example: bash scripts/nats-fixture.sh --count 50 --interval 2
Ctrl-C stops this fixture's server. Data and logs remain in the printed temp folder.
Each invocation creates fresh streams; use a new pipeline for a fresh fixture.
USAGE
}
fail() { echo "nats fixture: $*" >&2; exit 1; }
port=14223
count=20
interval=0
while (($#)); do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --port|--count|--interval)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --port) port="$2" ;;
        --count) count="$2" ;;
        --interval) interval="$2" ;;
      esac
      shift 2 ;;
    *) fail "unknown argument: $1 (see --help)" ;;
  esac
done
for value in "$port" "$count" "$interval"; do
  [[ "$value" =~ ^(0|[1-9][0-9]{0,5})$ ]] || fail "options must be nonnegative integers of at most 6 digits"
done
((port > 0 && port <= 65535)) || fail "port must be between 1 and 65535"
((count > 0)) || fail "count must be positive"
for tool in nats-server nats; do
  command -v "$tool" >/dev/null || fail "install $tool and put it on PATH"
done

# Never inherit the engine's account, URL, or JetStream namespace.
for variable in $(compgen -v NATS_); do unset "$variable"; done
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/filament-nats-fixture.XXXXXX")
url="nats://127.0.0.1:$port"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
nats-server -js -a 127.0.0.1 -p "$port" -sd "$fixture_dir/data" >"$fixture_dir/server.log" 2>&1 &
server_pid=$!

# Wait for our process's readiness, not an unrelated server already on this port.
ready=false
for ((attempt=0; attempt<100; attempt++)); do
  if ! kill -0 "$server_pid" 2>/dev/null; then
    cat "$fixture_dir/server.log" >&2
    fail "server failed to start (port $port may be in use)"
  fi
  if grep -q 'Server is ready' "$fixture_dir/server.log"; then
    ready=true
    break
  fi
  sleep 0.1
done
[[ "$ready" == true ]] || fail "server startup timed out; see $fixture_dir/server.log"
cli() { nats --no-context --server "$url" --timeout 3s "$@"; }
add_stream() {
  cli stream add "$1" --subjects "$2" --storage file --retention limits \
    --replicas 1 --discard new --max-bytes 256MiB --defaults >>"$fixture_dir/seed.log" 2>&1
}
add_stream ORDERS_US 'orders.us.>'
add_stream ORDERS_EU 'orders.eu.>'
add_stream PRODUCTS 'products.>'
add_stream INVENTORY 'inventory.>'
add_stream CUSTOMERS 'customers.>'

# JetStream publish acknowledgements ensure every printed seed count is stored.
subjects=(orders.us.created orders.eu.created products.created products.updated inventory.adjusted customers.created)
publish_round() {
  local subject
  for subject in "${subjects[@]}"; do
    cli pub "$subject" --jetstream --count "$1" \
      --header 'Content-Type:application/json' --header 'X-Fixture:filament' \
      "{\"event_id\":\"{{ID}}\",\"event_type\":\"$subject\",\"sequence\":{{Count}},\"occurred_at\":\"{{TimeStamp}}\",\"payload\":{\"sku\":\"demo-001\",\"quantity\":2,\"amount_cents\":2499}}" \
      >>"$fixture_dir/seed.log" 2>&1 || { tail -20 "$fixture_dir/seed.log" >&2; return 1; }
  done
}
publish_round "$count"
cat <<INFO
JetStream fixture ready
  Source URL: $url
  Resources:  orders.>  products.>  inventory.>  customers.>
  Seeded:     $((count * 6)) messages across 5 physical streams
  Logs/data:  $fixture_dir

Create a continuous NATS source using this URL, then add each desired resource
as a chip. Filament creates the consumers; none are pre-created by this fixture.
Local workers can connect at the URL above. Ctrl-C stops the fixture.
INFO
if ((interval > 0)); then
  echo "Publishing 6 more messages every $interval seconds."
  while kill -0 "$server_pid" 2>/dev/null; do
    sleep "$interval"
    publish_round 1
  done
  fail "server stopped unexpectedly"
else
  wait "$server_pid"
fi
