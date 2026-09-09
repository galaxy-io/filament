#!/usr/bin/env bash
# Shared engine/endpoint setup for Compose, image builds, and Testcontainers.
set -euo pipefail

fail() { echo "container setup: $*" >&2; exit 1; }
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export CONTAINER_ENGINE="${CONTAINER_ENGINE:-docker}"
case "$CONTAINER_ENGINE" in docker|podman) ;; *) fail "CONTAINER_ENGINE must be docker or podman" ;; esac
command -v "$CONTAINER_ENGINE" >/dev/null || fail "$CONTAINER_ENGINE is not installed or is not on PATH"

if [[ "$CONTAINER_ENGINE" == podman ]]; then
    if [[ "$(uname -s)" == Darwin ]]; then
        machine="${PODMAN_MACHINE:-$(podman machine list --format '{{if .Running}}{{.Name}}{{end}}' | sed '/^$/d')}"
        [[ -n "$machine" ]] || fail "start a Podman machine with podman machine start"
        machine_socket="$(podman machine inspect "$machine" --format '{{.ConnectionInfo.PodmanSocket.Path}}')"
        if [[ -n "${DOCKER_HOST:-}" && "$DOCKER_HOST" != "unix://$machine_socket" ]]; then
            fail "DOCKER_HOST does not match Podman machine $machine; unset it or select PODMAN_MACHINE"
        fi
        export DOCKER_HOST="unix://$machine_socket"
    else
        export DOCKER_HOST="${DOCKER_HOST:-unix://${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/podman/podman.sock}"
    fi
    # The native client and Docker-compatible API clients must use the same engine.
    export CONTAINER_HOST="$DOCKER_HOST"
    unset DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH
    rootless="$(podman info --format '{{.Host.Security.Rootless}}')" || fail "cannot reach Podman at $DOCKER_HOST; start the machine or podman.socket service"
    if [[ "$(uname -s)" == Darwin ]]; then
        export TESTCONTAINERS_HOST_OVERRIDE="${TESTCONTAINERS_HOST_OVERRIDE:-localhost}"
        if [[ "$rootless" == false ]]; then
            # Ryuk mounts a socket inside Linux, not the forwarded macOS socket.
            export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-/run/podman/podman.sock}"
            export TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED="${TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED:-true}"
        fi
    fi
else
    # Respect Docker contexts when tests are launched outside the Docker CLI.
    if [[ -n "${DOCKER_CONTEXT:-}" || -z "${DOCKER_HOST:-}" ]]; then
        export DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
    fi
    components="$(docker version --format '{{range .Server.Components}}{{.Name}} {{end}}')" || fail "cannot reach Docker; start the selected Docker engine"
    [[ "$components" != *"Podman Engine"* ]] || fail "the selected Docker endpoint is Podman; set CONTAINER_ENGINE=podman"
fi

action="${1:-doctor}"
shift || true
case "$action" in
    compose)
        cd "$repo_dir"
        env_files=(--env-file tests/testcontainers/internal/envfile/images.env)
        [[ ! -f docker/.env ]] || env_files+=(--env-file docker/.env)
        [[ ! -f .env ]] || env_files+=(--env-file .env)
        if [[ "$CONTAINER_ENGINE" == podman ]]; then
            export PODMAN_COMPOSE_PROVIDER="${PODMAN_COMPOSE_PROVIDER:-$(command -v docker-compose || true)}"
            [[ -n "$PODMAN_COMPOSE_PROVIDER" ]] || fail "install Docker Compose or set PODMAN_COMPOSE_PROVIDER to a tested provider"
        fi
        exec "$CONTAINER_ENGINE" compose "${env_files[@]}" "$@"
        ;;
    build) exec "$CONTAINER_ENGINE" build "$@" ;;
    exec)
        if [[ "$CONTAINER_ENGINE" == podman && "$(uname -s)" == Darwin && "$rootless" == true ]]; then
            fail "macOS tests require a rootful Podman machine for Ryuk; stop the machine, set --rootful, and restart it"
        fi
        exec "$@"
        ;;
    doctor)
        echo "Engine: $CONTAINER_ENGINE"
        echo "API endpoint: $DOCKER_HOST"
        if [[ "$CONTAINER_ENGINE" == podman ]]; then
            echo "Rootless: $rootless"
            echo "Ryuk socket: ${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-${DOCKER_HOST#unix://}}"
        fi
        ;;
    *) fail "expected compose, build, exec, or doctor" ;;
esac
