#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
./stop.sh && ./start.sh
