#!/bin/bash
cd "$(dirname "$0")" || exit 1
echo "Building..."
go build -o imgen ./cmd/imgen/
echo "Done. Binary: ./imgen"
