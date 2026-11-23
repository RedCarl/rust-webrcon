#!/usr/bin/env bash
[ -n "${BASH_VERSION:-}" ] || exec bash "$0" "$@"
set -euo pipefail

APP_DIR="${APP_DIR:-/app}"
LGSM_SCRIPT="${LGSM_SCRIPT:-$APP_DIR/rustserver}"
COMMON_CFG="${COMMON_CFG:-/data/config-lgsm/rustserver/common.cfg}"
SERVER_IDENTITY="${SERVER_IDENTITY:-rustserver}"
SERVER_LEVEL="${SERVER_LEVEL:-Procedural Map}"
SERVER_WORLDSIZE="${SERVER_WORLDSIZE:-4250}"
SERVER_SEED="${SERVER_SEED:-}"
SEED_SOURCE="${SEED_SOURCE:-rustmaps}"
MAP_WAIT_TIMEOUT="${MAP_WAIT_TIMEOUT:-600}"
MAP_WAIT_INTERVAL="${MAP_WAIT_INTERVAL:-5}"
S3_PREFIX="${S3_PREFIX:-rust/maps}"
S3_HOST="${S3_HOST:-}"
S3_PORT="${S3_PORT:-}"
S3_SECURE="${S3_SECURE:-}"
S3_ENDPOINT_URL="${S3_ENDPOINT_URL:-}"
RUSTMAPS_SEARCH_URL="${RUSTMAPS_SEARCH_URL:-https://api.rustmaps.com/internal/v1/maps/search?customMaps=false&sortBy=&source=all&staging=false&orgId=all&page=0}"
RUSTMAPS_SEARCH_BODY="${RUSTMAPS_SEARCH_BODY:-}"
RUSTMAPS_SEARCH_BODY_FILE="${RUSTMAPS_SEARCH_BODY_FILE:-}"
RUSTMAPS_TIMEOUT="${RUSTMAPS_TIMEOUT:-20}"
WEBRCON_API_URL="${WEBRCON_API_URL:-https://rust-webrcon.carljoy.cn}"
WEBRCON_PASSWORD="${WEBRCON_PASSWORD:-}"
WEBRCON_SERVER_ID="${WEBRCON_SERVER_ID:-1}"
STATE_FILE="${STATE_FILE:-/data/wipe-state.env}"
LOG_FILE="${LOG_FILE:-/data/wipe.log}"
CONTENT_TYPE="${CONTENT_TYPE:-application/octet-stream}"
SNAPSHOT_FILE=""
WIPE_SUCCESS=0
EXIT_STATUS=0

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

fail() {
    EXIT_STATUS=1
    log "失败: $*"
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1"
}

require_env() {
    [ -n "${!1:-}" ] || fail "缺少环境变量: $1"
}

urlencode_path() {
    local value="$1"
    local encoded=""
    local i char hex

    for ((i = 0; i < ${#value}; i++)); do
        char="${value:i:1}"
        case "$char" in
            [a-zA-Z0-9.~_/-]) encoded+="$char" ;;
            ' ') encoded+='%20' ;;
            '"') encoded+='%22' ;;
            '#') encoded+='%23' ;;
            '?') encoded+='%3F' ;;
            '&') encoded+='%26' ;;
            '+') encoded+='%2B' ;;
            '%') encoded+='%25' ;;
            *)
                printf -v hex '%02X' "'$char"
                encoded+="%$hex"
                ;;
        esac
    done

    echo "$encoded"
}

normalize_prefix() {
    local prefix="${1:-}"
    prefix="${prefix#/}"
    prefix="${prefix%/}"
    echo "$prefix"
}

normalize_bool() {
    local value="${1:-}"
    value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
    case "$value" in
        1|true|yes|on) echo "true" ;;
        0|false|no|off|"") echo "false" ;;
        *) fail "非法布尔值: $1" ;;
    esac
}

normalize_seed_source() {
    local value="${1:-rustmaps}"
    value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
    case "$value" in
        rustmaps) echo "$value" ;;
        *) fail "非法种子来源: $1" ;;
    esac
}

get_file_mtime() {
    stat -c '%Y' "$1" 2>/dev/null || stat -f '%m' "$1"
}

get_file_size() {
    stat -c '%s' "$1" 2>/dev/null || stat -f '%z' "$1"
}

get_s3_endpoint_url() {
    local secure protocol

    if [ -n "$S3_ENDPOINT_URL" ]; then
        echo "${S3_ENDPOINT_URL%/}"
        return
    fi

    if [ -z "$S3_HOST" ]; then
        echo ""
        return
    fi

    secure="$(normalize_bool "$S3_SECURE")"
    if [ "$secure" = "true" ]; then
        protocol="https"
    else
        protocol="http"
    fi

    if [ -n "$S3_PORT" ]; then
        echo "$protocol://$S3_HOST:$S3_PORT"
    else
        echo "$protocol://$S3_HOST"
    fi
}

is_loopback_host() {
    case "${1:-}" in
        127.*|localhost|0.0.0.0) return 0 ;;
        *) return 1 ;;
    esac
}

validate_public_url_config() {
    local endpoint_url="$1"

    if [ -n "${PUBLIC_BASE_URL:-}" ] || [ -z "$endpoint_url" ]; then
        return
    fi

    if is_loopback_host "$S3_HOST"; then
        fail "S3 endpoint 是本机地址 ${S3_HOST}，必须额外提供玩家可访问的 PUBLIC_BASE_URL"
    fi
}

aws_s3_cp() {
    local src="$1"
    local dst="$2"
    local endpoint_url="$3"

    if [ -n "$endpoint_url" ]; then
        aws --endpoint-url "$endpoint_url" s3 cp "$src" "$dst" --content-type "$CONTENT_TYPE"
    else
        aws s3 cp "$src" "$dst" --content-type "$CONTENT_TYPE"
    fi
}

aws_head_object() {
    local bucket="$1"
    local key="$2"
    local endpoint_url="$3"

    if [ -n "$endpoint_url" ]; then
        aws --endpoint-url "$endpoint_url" s3api head-object --bucket "$bucket" --key "$key"
    else
        aws s3api head-object --bucket "$bucket" --key "$key"
    fi
}

build_default_rustmaps_body() {
    printf '{"searchQuery":{"size":{"min":%s,"max":%s}}}' "$SERVER_WORLDSIZE" "$SERVER_WORLDSIZE"
}

read_rustmaps_request_body() {
    if [ -n "$RUSTMAPS_SEARCH_BODY_FILE" ]; then
        [ -f "$RUSTMAPS_SEARCH_BODY_FILE" ] || fail "RUSTMAPS_SEARCH_BODY_FILE 不存在: $RUSTMAPS_SEARCH_BODY_FILE"
        cat "$RUSTMAPS_SEARCH_BODY_FILE"
        return
    fi

    if [ -n "$RUSTMAPS_SEARCH_BODY" ]; then
        printf '%s' "$RUSTMAPS_SEARCH_BODY"
        return
    fi

    build_default_rustmaps_body
}

fetch_rustmaps_seed() {
    local request_body="$1"
    local previous_seed="${2:-}"
    local response

    response="$(
        curl -fsSL "$RUSTMAPS_SEARCH_URL" \
            -H 'accept: */*' \
            -H 'content-type: application/json' \
            -H 'origin: https://rustmaps.com' \
            -H 'referer: https://rustmaps.com/' \
            -H 'user-agent: Mozilla/5.0' \
            --max-time "$RUSTMAPS_TIMEOUT" \
            --data-raw "$request_body"
    )" || return 1

    RESPONSE_JSON="$response" python3 - "$previous_seed" <<'PY'
import json
import random
import os
import sys

previous_seed = sys.argv[1].strip()
payload = json.loads(os.environ["RESPONSE_JSON"])
items = payload.get("data") or []
seeds = []

for item in items:
    seed = item.get("seed")
    size = item.get("size")
    if isinstance(seed, int) and str(size) != "":
        seeds.append(seed)

if previous_seed:
    seeds = [seed for seed in seeds if str(seed) != previous_seed]

if not seeds:
    sys.exit(1)

print(random.choice(seeds))
PY
}

select_server_seed() {
    local seed_source="$1"
    local previous_seed="${2:-}"

    case "$seed_source" in
        rustmaps)
            local request_body
            request_body="$(read_rustmaps_request_body)"
            log "从 RustMaps 拉取候选种子..." >&2
            fetch_rustmaps_seed "$request_body" "$previous_seed" || fail "RustMaps 种子获取失败"
            ;;
    esac
}

write_common_cfg_var() {
    local key="$1"
    local value="$2"
    local escaped_value
    mkdir -p "$(dirname "$COMMON_CFG")"
    escaped_value="$(printf '%s' "$value" | sed 's/[&|\\]/\\&/g')"

    if [ -f "$COMMON_CFG" ] && grep -qE "^[[:space:]]*${key}=" "$COMMON_CFG"; then
        sed -i.bak -E "s|^[[:space:]]*${key}=.*$|${key}=\"${escaped_value}\"|" "$COMMON_CFG"
    else
        printf '\n%s="%s"\n' "$key" "$value" >> "$COMMON_CFG"
    fi
}

find_server_dir() {
    if [ -n "${RUST_SERVER_DIR:-}" ]; then
        [ -d "$RUST_SERVER_DIR" ] || fail "RUST_SERVER_DIR 不存在: $RUST_SERVER_DIR"
        echo "$RUST_SERVER_DIR"
        return
    fi

    local candidates=(
        "/data/serverfiles/server/$SERVER_IDENTITY"
        "$APP_DIR/serverfiles/server/$SERVER_IDENTITY"
        "$APP_DIR/server/$SERVER_IDENTITY"
        "$APP_DIR/serverfiles/$SERVER_IDENTITY"
        "$APP_DIR/serverfiles/server"
    )

    local dir
    for dir in "${candidates[@]}"; do
        if [ -d "$dir" ]; then
            echo "$dir"
            return
        fi
    done

    fail "无法定位地图目录，请设置 RUST_SERVER_DIR，例如 /app/serverfiles/server/$SERVER_IDENTITY"
}

snapshot_map_files() {
    local server_dir="$1"
    local snapshot_file="$2"

    : > "$snapshot_file"

    find "$server_dir" -type f -name '*.map' -print0 2>/dev/null \
        | while IFS= read -r -d '' file; do
            printf '%s\t%s\t%s\n' "$file" "$(get_file_mtime "$file")" "$(get_file_size "$file")"
        done > "$snapshot_file"
}

find_new_map_file() {
    local server_dir="$1"
    local started_at="$2"
    local snapshot_file="$3"

    find "$server_dir" -type f -name '*.map' -print0 2>/dev/null \
        | while IFS= read -r -d '' file; do
            printf '%s\t%s\t%s\n' "$(get_file_mtime "$file")" "$(get_file_size "$file")" "$file"
        done \
        | awk -F '\t' -v started_at="$started_at" -v snapshot_file="$snapshot_file" '
            BEGIN {
                while ((getline line < snapshot_file) > 0) {
                    split(line, parts, "\t")
                    baseline[parts[1]] = parts[2] "\t" parts[3]
                }
                close(snapshot_file)
            }

            {
                mtime = $1 + 0
                size = $2 + 0
                file = $3
                current = mtime "\t" size

                if (size <= 0 || mtime < started_at) {
                    next
                }

                if (!(file in baseline) || baseline[file] != current) {
                    print $0
                }
            }
        ' \
        | sort -t $'\t' -k1,1nr \
        | head -n 1 \
        | cut -f3-
}

wait_for_map_file() {
    local server_dir="$1"
    local started_at="$2"
    local snapshot_file="$3"
    local deadline=$((SECONDS + MAP_WAIT_TIMEOUT))
    local map_file=""

    while [ "$SECONDS" -lt "$deadline" ]; do
        map_file="$(find_new_map_file "$server_dir" "$started_at" "$snapshot_file")"
        if [ -n "$map_file" ]; then
            echo "$map_file"
            return
        fi

        sleep "$MAP_WAIT_INTERVAL"
    done

    return 1
}

safe_stop_server() {
    log "停止服务器..."
    "$LGSM_SCRIPT" stop >> "$LOG_FILE" 2>&1 || true
}

run_full_wipe() {
    log "执行 LinuxGSM full wipe..."
    "$LGSM_SCRIPT" fw >> "$LOG_FILE" 2>&1
}

send_runtime_levelurl() {
    local map_url="$1"
    local payload

    require_command curl
    [ -n "$WEBRCON_API_URL" ] || fail "缺少环境变量: WEBRCON_API_URL"
    [ -n "$WEBRCON_PASSWORD" ] || fail "缺少环境变量: WEBRCON_PASSWORD"

    payload="$(
        python3 - "$map_url" "$WEBRCON_PASSWORD" <<'PY'
import json
import sys

print(json.dumps({
    "command": f'server.levelurl "{sys.argv[1]}"',
    "password": sys.argv[2],
}, ensure_ascii=False))
PY
    )"

    curl -fsSL -X POST \
        "${WEBRCON_API_URL%/}/api/servers/${WEBRCON_SERVER_ID}/command" \
        -H 'Content-Type: application/json' \
        -d "$payload" >> "$LOG_FILE" 2>&1
}

s3_object_exists() {
    local s3_key="$1"
    local endpoint_url="${2:-}"
    aws_head_object "$S3_BUCKET" "$s3_key" "$endpoint_url" >/dev/null 2>&1
}

build_public_url() {
    local s3_key="$1"
    local endpoint_url="$2"
    local encoded_key
    encoded_key="$(urlencode_path "$s3_key")"

    if [ -n "${PUBLIC_BASE_URL:-}" ]; then
        echo "${PUBLIC_BASE_URL%/}/$encoded_key"
    elif [ -n "$endpoint_url" ]; then
        echo "${endpoint_url}/${S3_BUCKET}/$encoded_key"
    else
        require_env AWS_REGION
        echo "https://${S3_BUCKET}.s3.${AWS_REGION}.amazonaws.com/$encoded_key"
    fi
}

cleanup() {
    local exit_code=$?
    if [ "${EXIT_STATUS:-0}" -ne 0 ]; then
        exit_code="$EXIT_STATUS"
    fi

    if [ "$WIPE_SUCCESS" -ne 1 ]; then
        safe_stop_server
        log "wipe 未完成。"
    fi

    if [ -n "$SNAPSHOT_FILE" ] && [ -f "$SNAPSHOT_FILE" ]; then
        rm -f "$SNAPSHOT_FILE"
    fi

    exit "$exit_code"
}

main() {
    require_command aws
    require_command awk
    require_command find
    require_command grep
    require_command mktemp
    require_command python3
    require_command sed
    require_command stat
    require_env S3_BUCKET

    [ -x "$LGSM_SCRIPT" ] || fail "LGSM 脚本不可执行: $LGSM_SCRIPT"
    trap cleanup EXIT

    if [ -z "$SERVER_SEED" ]; then
        local previous_seed=""
        if [ -f "$STATE_FILE" ]; then
            previous_seed="$(bash -c 'source "$1" >/dev/null 2>&1 || exit 0; printf "%s" "${MAP_SEED:-}"' bash "$STATE_FILE" 2>/dev/null || true)"
        fi
        SERVER_SEED="$(select_server_seed "$(normalize_seed_source "$SEED_SOURCE")" "$previous_seed")"
    fi

    local endpoint_url
    endpoint_url="$(get_s3_endpoint_url)"
    validate_public_url_config "$endpoint_url"

    local server_dir
    server_dir="$(find_server_dir)"
    log "地图目录: $server_dir"
    log "本次地图参数: level=$SERVER_LEVEL worldsize=$SERVER_WORLDSIZE seed=$SERVER_SEED"

    log "写入 LinuxGSM common.cfg 种子配置..."
    write_common_cfg_var "seed" "$SERVER_SEED"
    write_common_cfg_var "worldsize" "$SERVER_WORLDSIZE"

    SNAPSHOT_FILE="$(mktemp)"
    snapshot_map_files "$server_dir" "$SNAPSHOT_FILE"

    local started_at
    started_at="$(date +%s)"
    run_full_wipe

    log "等待服务端生成新的 .map 文件..."
    local map_file
    if ! map_file="$(wait_for_map_file "$server_dir" "$started_at" "$SNAPSHOT_FILE")"; then
        fail "超时未找到本次新生成的 .map 文件"
    fi

    local map_filename
    map_filename="$(basename "$map_file")"
    local s3_prefix
    s3_prefix="$(normalize_prefix "$S3_PREFIX")"

    local s3_key
    if [ -n "$s3_prefix" ]; then
        s3_key="$s3_prefix/$SERVER_IDENTITY/$map_filename"
    else
        s3_key="$SERVER_IDENTITY/$map_filename"
    fi

    if s3_object_exists "$s3_key" "$endpoint_url"; then
        fail "S3 已存在同名对象，拒绝覆盖历史地图: s3://$S3_BUCKET/$s3_key"
    fi

    log "上传地图到 S3: $map_filename -> s3://$S3_BUCKET/$s3_key"
    aws_s3_cp "$map_file" "s3://$S3_BUCKET/$s3_key" "$endpoint_url" >> "$LOG_FILE" 2>&1

    log "校验 S3 对象存在..."
    aws_head_object "$S3_BUCKET" "$s3_key" "$endpoint_url" >> "$LOG_FILE" 2>&1

    local map_url
    map_url="$(build_public_url "$s3_key" "$endpoint_url")"
    log "地图 URL: $map_url"

    log "通过运行时命令更新 server.levelurl..."
    send_runtime_levelurl "$map_url"

    mkdir -p "$(dirname "$STATE_FILE")"
    cat > "$STATE_FILE" <<EOF
MAP_FILENAME="$map_filename"
MAP_LOCAL_PATH="$map_file"
MAP_S3_KEY="$s3_key"
MAP_URL="$map_url"
MAP_SEED="$SERVER_SEED"
MAP_SIZE="$SERVER_WORLDSIZE"
GENERATED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
EOF

    WIPE_SUCCESS=1
    log "wipe 完成，正式服已使用 levelurl 启动。"
}

main "$@"
