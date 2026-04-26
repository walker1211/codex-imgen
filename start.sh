#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

is_imgen_serve_process() {
    local pid="$1"
    local command
    command=$(ps -p "$pid" -o command= 2>/dev/null || true)

    [[ "$command" == *"serve"* ]] && \
        { [[ "$command" == *"$(pwd)/imgen"* ]] || [[ "$command" == *"./imgen"* ]] || [[ "$command" == *"imgen"* ]]; }
}

mkdir -p logs
PID_FILE="logs/imgen.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(<"$PID_FILE")
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        if is_imgen_serve_process "$PID"; then
            echo "imgen serve is already running (PID: $PID)"
            exit 1
        fi
        rm -f "$PID_FILE"
    else
        rm -f "$PID_FILE"
    fi
fi

nohup ./imgen serve > logs/out.log 2>&1 &
PID=$!
echo "$PID" > "$PID_FILE"
echo "imgen serve started (PID: $PID)"
echo "Logs: logs/out.log"
