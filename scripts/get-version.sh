#!/bin/bash

set -euo pipefail

TAG=$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null)

if [ -z "$TAG" ]; then
  echo -n ""
  exit 1
fi

echo -n "${TAG}"

COMMIT_COUNT=$(git rev-list --count "${TAG}..HEAD" 2>/dev/null || echo "0")

if [ "$COMMIT_COUNT" != "0" ]; then
  echo -n "+${COMMIT_COUNT}"
fi
