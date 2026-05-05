#!/bin/bash
set -euo pipefail
exec docker run --rm \
  -v "$(pwd):/work" \
  -w /work \
  chocolatey/choco choco "$@"
