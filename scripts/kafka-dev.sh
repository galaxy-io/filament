#!/usr/bin/env bash
# Local Kafka fixture; only Bash and Docker Compose (or Podman Compose) required.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
compose() {
    bash "$script_dir/container.sh" compose --project-name filament-kafka-dev \
        -f "$script_dir/kafka-dev.compose.yaml" "$@"
}
fail() { echo "kafka-dev: $*" >&2; exit 1; }
usage() {
    cat <<'EOF'
Usage: bash scripts/kafka-dev.sh [run|seed|down|reset]

  run    Start Kafka, seed records, then publish periodically (default).
  seed   Start Kafka and publish only the seed records.
  down   Stop Kafka, preserving its data volume.
  reset  Stop Kafka and DELETE this fixture's data volume.

Environment:
  KAFKA_TOPIC       Topic name (default: orders)
  KAFKA_PORT        Local broker port (default: 9092)
  KAFKA_PARTITIONS  Partitions when creating the topic (default: 3)
  SEED_COUNT        Records added on each invocation (default: 100; 0 skips)
  BATCH_SIZE        Records per periodic batch (default: 10)
  INTERVAL_SECONDS Delay between batches, in whole seconds (default: 5)
  KAFKA_IMAGE      Override the repository's pinned Kafka image
  CONTAINER_ENGINE docker (default) or podman

Ctrl-C stops publishing and leaves Kafka running. Run 'down' to stop it.
EOF
}
action="${1:-run}"
[[ $# -le 1 ]] || fail "expected at most one action (see --help)"
case "$action" in
    -h|--help|help) usage; exit 0 ;;
    down) compose down; exit 0 ;;
    reset) compose down --volumes; exit 0 ;;
    run|seed) ;;
    *) fail "unknown action: $action (see --help)" ;;
esac

topic="${KAFKA_TOPIC:-orders}"
export KAFKA_PORT="${KAFKA_PORT:-9092}"
partitions="${KAFKA_PARTITIONS:-3}"
seed_count="${SEED_COUNT:-100}"
batch_size="${BATCH_SIZE:-10}"
interval="${INTERVAL_SECONDS:-5}"
[[ "$topic" =~ ^[a-zA-Z0-9._-]+$ && ${#topic} -le 249 && "$topic" != . && "$topic" != .. ]] || fail "invalid KAFKA_TOPIC"
# Reject leading zeroes so Bash arithmetic never interprets input as octal.
for value in "$KAFKA_PORT" "$partitions" "$batch_size" "$interval"; do
    [[ "$value" =~ ^[1-9][0-9]*$ && ${#value} -le 9 ]] || fail "port, partitions, batch size, and interval must be positive integers (at most 9 digits)"
done
[[ "$seed_count" =~ ^(0|[1-9][0-9]*)$ && ${#seed_count} -le 9 ]] || fail "SEED_COUNT must be a nonnegative integer (at most 9 digits)"
(( KAFKA_PORT <= 65535 )) || fail "KAFKA_PORT must be at most 65535"

trap 'printf "\nPublisher stopped; Kafka remains running. Stop it with: bash scripts/kafka-dev.sh down\n"; exit 0' INT TERM
compose up -d --wait --wait-timeout 180
compose exec -T kafka /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:19092 --create --if-not-exists \
    --topic "$topic" --partitions "$partitions" --replication-factor 1

cat <<EOF

Kafka is ready (connect from a worker running on this host):
  Broker / instance address: localhost:$KAFKA_PORT
  Topic:                     $topic
  TLS:                       disabled
  Authentication / SASL:     none (no username or password)
  Start position:            earliest

Filament source configuration:
{"brokers":["localhost:$KAFKA_PORT"],"topics":["$topic"],"tls_enabled":false,"sasl_mechanism":"none","start_position":"earliest"}

EOF

# A run prefix avoids colliding keys when seeding an existing topic again.
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
sequence=0
publish() {
    local count="$1" phase="$2" timestamp i n
    (( count > 0 )) || return 0
    timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    # Tab separates the Kafka key from its JSON value; the producer hashes keys
    # across partitions. Kafka's own record timestamp is also set by the producer.
    for ((i = 1; i <= count; i++)); do
        n=$((sequence + i))
        printf '%s-%s\t{"event_id":"%s-%s","sequence":%s,"phase":"%s","customer_id":"customer-%s","amount_cents":%s,"status":"created","created_at":"%s"}\n' \
            "$run_id" "$n" "$run_id" "$n" "$n" "$phase" "$((n % 25 + 1))" "$((1000 + n % 9000))" "$timestamp"
    done | compose exec -T kafka /opt/kafka/bin/kafka-console-producer.sh \
        --bootstrap-server localhost:19092 --topic "$topic" \
        --property parse.key=true --property $'key.separator=\t' \
        --producer-property acks=all --producer-property delivery.timeout.ms=30000 \
        --producer-property request.timeout.ms=10000 --sync
    sequence=$((sequence + count))
    printf '%s Published %s %s records to %s (this run: %s)\n' "$timestamp" "$count" "$phase" "$topic" "$sequence"
}

publish "$seed_count" seed
[[ "$action" == run ]] || exit 0
printf 'Publishing %s records every %s seconds. Ctrl-C stops publishing.\n' "$batch_size" "$interval"
while true; do
    sleep "$interval"
    publish "$batch_size" live
done
