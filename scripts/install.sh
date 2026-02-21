#!/usr/bin/env bash
set -euo pipefail

APP_NAME="netcheck"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"

PREFIX="${HOME}/.local"
BIN_DIR=""
SOURCE_BIN=""
NO_PATH_UPDATE=0

usage() {
  cat <<USAGE
Usage: scripts/install.sh [options]

Build and install ${APP_NAME}.

Options:
  --prefix <dir>       Install prefix (default: \$HOME/.local)
  --bin-dir <dir>      Exact destination directory for the binary
  --source-bin <path>  Copy an existing binary instead of building
  --no-path-update     Do not update shell profile when dir is not on PATH
  -h, --help           Show this help
USAGE
}

die() {
  echo "install.sh: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      [[ $# -ge 2 ]] || die "missing value for --prefix"
      PREFIX="$2"
      shift 2
      ;;
    --bin-dir)
      [[ $# -ge 2 ]] || die "missing value for --bin-dir"
      BIN_DIR="$2"
      shift 2
      ;;
    --source-bin)
      [[ $# -ge 2 ]] || die "missing value for --source-bin"
      SOURCE_BIN="$2"
      shift 2
      ;;
    --no-path-update)
      NO_PATH_UPDATE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

if [[ -z "${BIN_DIR}" ]]; then
  BIN_DIR="${PREFIX%/}/bin"
fi

mkdir -p "${BIN_DIR}"
DEST_BIN="${BIN_DIR%/}/${APP_NAME}"

if [[ -n "${SOURCE_BIN}" ]]; then
  [[ -f "${SOURCE_BIN}" ]] || die "source binary does not exist: ${SOURCE_BIN}"
  install -m 0755 "${SOURCE_BIN}" "${DEST_BIN}"
else
  command -v go >/dev/null 2>&1 || die "go is required to build ${APP_NAME}"
  TMP_BIN="$(mktemp "${TMPDIR:-/tmp}/${APP_NAME}.XXXXXX")"
  trap 'rm -f "${TMP_BIN}"' EXIT
  (
    cd "${ROOT_DIR}"
    GOCACHE="${ROOT_DIR}/.gocache" go build -o "${TMP_BIN}" ./cmd/netcheck
  )
  install -m 0755 "${TMP_BIN}" "${DEST_BIN}"
fi

in_path=0
case ":${PATH}:" in
  *":${BIN_DIR}:"*) in_path=1 ;;
esac

if [[ ${in_path} -eq 0 && ${NO_PATH_UPDATE} -eq 0 ]]; then
  shell_name="$(basename -- "${SHELL:-zsh}")"
  case "${shell_name}" in
    zsh) rc_file="${HOME}/.zshrc" ;;
    bash) rc_file="${HOME}/.bashrc" ;;
    *) rc_file="${HOME}/.profile" ;;
  esac

  export_line="export PATH=\"${BIN_DIR}:\$PATH\""
  touch "${rc_file}"
  if ! grep -Fqx "${export_line}" "${rc_file}"; then
    printf '\n%s\n' "${export_line}" >> "${rc_file}"
  fi
  export PATH="${BIN_DIR}:${PATH}"
  echo "Updated PATH in ${rc_file}"
fi

echo "Installed ${APP_NAME} to ${DEST_BIN}"
echo "Try: ${APP_NAME} man"
