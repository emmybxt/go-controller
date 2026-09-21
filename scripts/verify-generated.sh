#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
go run ./cmd/gocontroller-gen -dir ./example -out routes.gen.go -check
go run ./cmd/gocontroller-gen -dir ./example/declarations -out routes.gen.go -check
echo "Generated files are up to date."
