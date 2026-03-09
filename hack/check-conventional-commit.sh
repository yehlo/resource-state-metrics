#!/usr/bin/env bash
# This script validates commit messages according to the Conventional Commits spec: https://www.conventionalcommits.org
# compilerla/conventional-pre-commit was dropped because even when verbose logging was enabled, it did not provide enough information to debug why a commit message was rejected.

set -euo pipefail

ALLOWED_TYPES="build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test"

commit_msg=$(head -1 "$1")

# Extract the type (characters before an optional scope or colon).
type=$(echo "$commit_msg" | grep -oE '^[a-z]+' || true)

if [[ -z "$type" ]]; then
  echo "error: commit message must start with a type followed by a colon (e.g., 'feat: add new feature'), got: '$type'"
  echo "       allowed types: $(echo "$ALLOWED_TYPES" | tr '|' ' ')"
  exit 1
fi

if ! echo "$type" | grep -qE "^($ALLOWED_TYPES)$"; then
  echo "error: '$type' is not an allowed commit type."
  echo "       allowed types: $(echo "$ALLOWED_TYPES" | tr '|' ' ')"
  exit 1
fi

# Validate the full header matches: type[(scope)][!]: subject
if ! echo "$commit_msg" | grep -qE '^[a-z]+(\([^)]+\))?!?: .+'; then
  echo "error: commit header must follow the pattern: type[(scope)][!]: subject"
  echo "       allowed types: $(echo "$ALLOWED_TYPES" | tr '|' ' ')"
  exit 1
fi
