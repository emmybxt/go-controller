#!/usr/bin/env bash
set -euo pipefail

go generate ./...

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Not a git repository; generation completed."
  exit 0
fi

if ! git diff --quiet -- . ':(exclude)go.sum'; then
  echo "Generated files are out of date. Run: go generate ./... and commit the changes."
  git --no-pager diff --name-only -- . ':(exclude)go.sum'
  exit 1
fi

echo "Generated files are up to date."
