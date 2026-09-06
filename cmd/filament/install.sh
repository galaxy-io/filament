#!/bin/sh
# Installs the latest filament release. Served at https://getgalaxy.io/filament/install

set -eu

REPO="galaxy-io/filament"
INSTALL_DIR="${FILAMENT_INSTALL_DIR:-${HOME}/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "${OS}" in
  linux|darwin) ;;
  *) echo "Unsupported operating system: ${OS}" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ARCHIVE="filament_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/latest/download"
if [ -n "${FILAMENT_VERSION:-}" ]; then
  BASE="https://github.com/${REPO}/releases/download/${FILAMENT_VERSION}"
fi

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

echo "Downloading ${BASE}/${ARCHIVE}"
curl -fsSL "${BASE}/${ARCHIVE}" -o "${TMP}/${ARCHIVE}"
curl -fsSL "${BASE}/checksums.txt" -o "${TMP}/checksums.txt"

EXPECTED=$(grep " ${ARCHIVE}\$" "${TMP}/checksums.txt" | awk '{print $1}')
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP}/${ARCHIVE}" | awk '{print $1}')
else
  ACTUAL=$(shasum -a 256 "${TMP}/${ARCHIVE}" | awk '{print $1}')
fi
if [ -z "${EXPECTED}" ] || [ "${EXPECTED}" != "${ACTUAL}" ]; then
  echo "Checksum mismatch for ${ARCHIVE}" >&2
  exit 1
fi

tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}" filament
mkdir -p "${INSTALL_DIR}"
mv -f "${TMP}/filament" "${INSTALL_DIR}/filament"

echo "Installed ${INSTALL_DIR}/filament"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo
    echo "${INSTALL_DIR} is not in your PATH. Add it with"
    echo
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo
echo "Run filament --help to get started."
