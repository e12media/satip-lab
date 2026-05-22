#!/usr/bin/env bash
set -euo pipefail

test -x bin/satip-lab
test -x bin/satip-lab-mcp
test -x bin/satip-lab-smoke
test -x bin/satip-labctl
