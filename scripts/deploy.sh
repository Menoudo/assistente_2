#!/usr/bin/env bash
# Deploy waiting-mcp into a target directory.
set -euo pipefail

usage() {
	cat <<'EOF'
Usage: deploy.sh [options] <target-dir>

Deploy waiting-mcp binary and data layout into <target-dir>.

Layout:
  <target-dir>/
    bin/waiting-mcp
    README.md
    CHANGELOG.md
    data/waiting/
    data/waiting/done/
    data/people/

Options:
  --examples         Copy example markdown files from the repository
  --cursor-global    Add or update the "waiting" server in ~/.cursor/mcp.json
  --cursor-project   Write .cursor/mcp.json into <target-dir>
  --copy-binary      Copy ./waiting-mcp from the repository instead of building
  --force            Overwrite existing binary
  -h, --help         Show this help

Examples:
  ./scripts/deploy.sh ~/gtd/waiting-mcp
  ./scripts/deploy.sh --examples --cursor-global ~/gtd/waiting-mcp
EOF
}

log() {
	printf '[deploy] %s\n' "$*"
}

die() {
	printf '[deploy] error: %s\n' "$*" >&2
	exit 1
}

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

repo_root() {
	local script_dir
	script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	cd "${script_dir}/.." && pwd
}

resolve_path() {
	local path="$1"
	if command -v python3 >/dev/null 2>&1; then
		python3 -c 'import os, sys; print(os.path.abspath(os.path.expanduser(sys.argv[1])))' "$path"
	else
		cd "$(dirname "$path")" 2>/dev/null || die "cannot resolve path: $path"
		printf '%s/%s\n' "$(pwd)" "$(basename "$path")"
	fi
}

copy_examples() {
	local source_dir="$1"
	local target_dir="$2"
	if [[ ! -d "${source_dir}/waiting" || ! -d "${source_dir}/people" ]]; then
		die "example data not found in repository"
	fi
	cp -R "${source_dir}/waiting/." "${target_dir}/data/waiting/"
	cp -R "${source_dir}/people/." "${target_dir}/data/people/"
}

copy_release_docs() {
	local target_dir="$1"
	local readme="${ROOT}/README.md"
	local changelog="${ROOT}/CHANGELOG.md"
	[[ -f "${readme}" ]] || die "README not found in repository: ${readme}"
	[[ -f "${changelog}" ]] || die "CHANGELOG not found in repository: ${changelog}"
	cp "${readme}" "${target_dir}/README.md"
	cp "${changelog}" "${target_dir}/CHANGELOG.md"
}

write_project_cursor_config() {
	local target_dir="$1"
	local bin_path="${target_dir}/bin/waiting-mcp"
	local data_path="${target_dir}/data"
	mkdir -p "${target_dir}/.cursor"
	cat >"${target_dir}/.cursor/mcp.json" <<EOF
{
  "mcpServers": {
    "waiting": {
      "command": "${bin_path}",
      "args": ["--stdio"],
      "env": {
        "WAITING_DATA_DIR": "${data_path}"
      }
    }
  }
}
EOF
}

update_global_cursor_config() {
	local bin_path="$1"
	local data_path="$2"
	local config_path="${HOME}/.cursor/mcp.json"
	mkdir -p "${HOME}/.cursor"

	python3 - "${config_path}" "${bin_path}" "${data_path}" <<'PY'
import json
import os
import sys

config_path, command, data_dir = sys.argv[1:4]
entry = {
    "command": command,
    "args": ["--stdio"],
    "env": {"WAITING_DATA_DIR": data_dir},
}

if os.path.exists(config_path):
    with open(config_path, encoding="utf-8") as handle:
        config = json.load(handle)
else:
    config = {}

servers = config.setdefault("mcpServers", {})
servers["waiting"] = entry

with open(config_path, "w", encoding="utf-8") as handle:
    json.dump(config, handle, indent=2)
    handle.write("\n")
PY
}

prepare_build_env() {
	local limits=(65536 32768 16384 8192 4096)
	local limit
	for limit in "${limits[@]}"; do
		if ulimit -n "${limit}" 2>/dev/null; then
			log "file descriptor limit: $(ulimit -n)"
			return 0
		fi
	done
	log "warning: could not raise file descriptor limit"
}

build_binary() {
	prepare_build_env
	(
		cd "${ROOT}"
		GOMAXPROCS=1 go build -p 1 -trimpath -o "${BIN_PATH}" ./cmd/waiting-mcp
	)
}

copy_local_binary() {
	local source="${ROOT}/waiting-mcp"
	[[ -f "${source}" ]] || die "local binary not found: ${source} (run: go build -o waiting-mcp ./cmd/waiting-mcp)"
	install -m 755 "${source}" "${BIN_PATH}"
}

install_binary() {
	if [[ "${COPY_BINARY}" -eq 1 ]]; then
		log "copying local binary..."
		copy_local_binary
		return
	fi

	log "building waiting-mcp..."
	if build_binary; then
		return
	fi

	local local_binary="${ROOT}/waiting-mcp"
	if [[ -f "${local_binary}" ]]; then
		log "build failed, falling back to repository binary..."
		copy_local_binary
		return
	fi

	die "build failed (often caused by 'too many open files'); try: ulimit -n 8192, or build manually and rerun with --copy-binary"
}

WITH_EXAMPLES=0
CURSOR_GLOBAL=0
CURSOR_PROJECT=0
COPY_BINARY=0
FORCE=0
TARGET=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--examples)
		WITH_EXAMPLES=1
		shift
		;;
	--cursor-global)
		CURSOR_GLOBAL=1
		shift
		;;
	--cursor-project)
		CURSOR_PROJECT=1
		shift
		;;
	--force)
		FORCE=1
		shift
		;;
	--copy-binary)
		COPY_BINARY=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		die "unknown option: $1"
		;;
	*)
		if [[ -n "${TARGET}" ]]; then
			die "unexpected argument: $1"
		fi
		TARGET="$1"
		shift
		;;
	esac
done

[[ -n "${TARGET}" ]] || {
	usage
	exit 1
}

require_cmd python3
if [[ "${COPY_BINARY}" -ne 1 ]]; then
	require_cmd go
fi

ROOT="$(repo_root)"
TARGET_DIR="$(resolve_path "${TARGET}")"
BIN_PATH="${TARGET_DIR}/bin/waiting-mcp"

log "repository: ${ROOT}"
log "target: ${TARGET_DIR}"

mkdir -p "${TARGET_DIR}/bin" "${TARGET_DIR}/data/waiting/done" "${TARGET_DIR}/data/people"

if [[ -f "${BIN_PATH}" && "${FORCE}" -ne 1 ]]; then
	die "binary already exists: ${BIN_PATH} (use --force to overwrite)"
fi

install_binary

log "copying README and CHANGELOG..."
copy_release_docs "${TARGET_DIR}"

if [[ "${WITH_EXAMPLES}" -eq 1 ]]; then
	log "copying example data..."
	copy_examples "${ROOT}/data" "${TARGET_DIR}"
fi

if [[ "${CURSOR_PROJECT}" -eq 1 ]]; then
	log "writing project Cursor config..."
	write_project_cursor_config "${TARGET_DIR}"
fi

if [[ "${CURSOR_GLOBAL}" -eq 1 ]]; then
	log "updating global Cursor config..."
	update_global_cursor_config "${BIN_PATH}" "${TARGET_DIR}/data"
fi

cat <<EOF

Готово.

  binary:    ${BIN_PATH}
  readme:    ${TARGET_DIR}/README.md
  changelog: ${TARGET_DIR}/CHANGELOG.md
  data:      ${TARGET_DIR}/data

Запуск:
  WAITING_DATA_DIR="${TARGET_DIR}/data" "${BIN_PATH}" --stdio

HTTP:
  WAITING_DATA_DIR="${TARGET_DIR}/data" WAITING_MCP_TOKEN=secret "${BIN_PATH}" --http :8080

EOF

if [[ "${CURSOR_GLOBAL}" -eq 0 && "${CURSOR_PROJECT}" -eq 0 ]]; then
	cat <<EOF
Cursor MCP snippet:
  command: ${BIN_PATH}
  WAITING_DATA_DIR: ${TARGET_DIR}/data

Повторите deploy с --cursor-global или --cursor-project для автоконфигурации.
EOF
fi

if [[ "${CURSOR_GLOBAL}" -eq 1 || "${CURSOR_PROJECT}" -eq 1 ]]; then
	cat <<'EOF'
Перезагрузите Cursor: Cmd+Shift+P → Developer: Reload Window
EOF
fi
