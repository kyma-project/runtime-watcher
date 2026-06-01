#!/bin/bash
input=$(cat)
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

[[ "$file_path" == *.go ]] || exit 0

# Skip vendor and e2e/integration test files
[[ "$file_path" == */vendor/* ]] && exit 0
[[ "$file_path" == */tests/e2e/* ]] && exit 0
[[ "$file_path" == */tests/integration/* ]] && exit 0

PROJECT_ROOT="$PWD"

# Determine which module the file belongs to and run lint from there
if [[ "$file_path" == "${PROJECT_ROOT}/listener/"* ]]; then
    MODULE_DIR="${PROJECT_ROOT}/listener"
elif [[ "$file_path" == "${PROJECT_ROOT}/runtime-watcher/"* ]]; then
    MODULE_DIR="${PROJECT_ROOT}/runtime-watcher"
else
    exit 0
fi

output=$(make -C "$MODULE_DIR" lint 2>&1)
exit_code=$?

if [[ $exit_code -ne 0 ]]; then
    echo "$output" | jq -Rs \
        '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: ("Lint issues that require manual fixes:\n" + .)}}'
    exit 1
fi
