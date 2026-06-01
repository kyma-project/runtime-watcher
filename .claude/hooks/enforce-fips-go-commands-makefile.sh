#!/usr/bin/env bash
set -euo pipefail

input=$(cat)

# For Edit, check new_string; for Write, check content
proposed=$(python3 -c "
import sys, json
d = json.load(sys.stdin)
ti = d.get('tool_input', {})
print(ti.get('new_string') or ti.get('content') or '')
" <<< "$input" 2>/dev/null || echo "")

if [[ -z "$proposed" ]]; then
    exit 0
fi

# Check recipe lines (tab-indented) that use bare 'go' without GOFIPS140=v1.0.0
violations=$(echo "$proposed" | grep -En $'^\t.*go[[:space:]]+(build|test|run|install)' | grep -v 'GOFIPS140=v1\.0\.0' || true)

if [[ -n "$violations" ]]; then
    echo "FIPS check failed: 'go' used without 'GOFIPS140=v1.0.0' in Makefile recipe:" >&2
    echo "$violations" >&2
    echo "Prefix build, test, run, and install commands with 'GOFIPS140=v1.0.0 go'." >&2
    exit 2
fi
