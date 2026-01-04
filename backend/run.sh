#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BASE_DIR"

BIN="${BIN:-$BASE_DIR/server}"
CONFIG_PATH="${CONFIG:-../config.json}"

if [[ ! -f "$BIN" ]]; then
  echo "未找到可执行文件：$BIN"
  echo "请先编译：go build -o \"$BIN\" ."
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  chmod +x "$BIN" 2>/dev/null || true
fi

CMD=("$BIN" -config "$CONFIG_PATH")
PID_FILE="${PID_FILE:-$BASE_DIR/server.pid}"
LOG_DIR="${LOG_DIR:-$BASE_DIR/logs}"
LOG_FILE="${LOG_FILE:-$LOG_DIR/backend.log}"

mkdir -p "$LOG_DIR"

is_running() {
  if [[ ! -f "$PID_FILE" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -z "$pid" ]]; then
    return 1
  fi
  kill -0 "$pid" 2>/dev/null
}

start() {
  if is_running; then
    echo "已在运行：PID=$(cat "$PID_FILE")"
    exit 0
  fi

  nohup "${CMD[@]}" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
  echo "启动成功：PID=$(cat "$PID_FILE")"
  echo "日志：$LOG_FILE"
}

stop() {
  if ! is_running; then
    rm -f "$PID_FILE"
    echo "未在运行"
    exit 0
  fi

  local pid
  pid="$(cat "$PID_FILE")"
  kill "$pid" 2>/dev/null || true
  for _ in {1..50}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$PID_FILE"
      echo "已停止"
      exit 0
    fi
    sleep 0.2
  done
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$PID_FILE"
  echo "已强制停止"
}

status() {
  if is_running; then
    echo "运行中：PID=$(cat "$PID_FILE")"
    exit 0
  fi
  echo "未运行"
}

restart() {
  stop || true
  start
}

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  *) echo "用法：$0 {start|stop|restart|status}" && exit 2 ;;
esac
