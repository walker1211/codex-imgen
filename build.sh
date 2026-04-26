#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")" || exit 1
echo "Building..."
go build -o imgen ./cmd/imgen/
go build -o skill-sync ./cmd/skill-sync/
echo "Done. Binaries: ./imgen ./skill-sync"
