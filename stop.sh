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

PID_FILE="logs/imgen.pid"

if [ ! -f "$PID_FILE" ]; then
    echo "imgen serve is not running"
    exit 0
fi

PID=$(<"$PID_FILE")

if [ -z "$PID" ] || ! kill -0 "$PID" 2>/dev/null; then
    rm -f "$PID_FILE"
    echo "imgen serve is not running"
    exit 0
fi

if ! is_imgen_serve_process "$PID"; then
    echo "PID $PID does not appear to be this repo's imgen serve process"
    exit 1
fi

kill "$PID"
rm -f "$PID_FILE"
echo "imgen serve stopped (PID: $PID)"
