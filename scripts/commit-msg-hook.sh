#!/bin/sh
# Rejects commit messages that carry tool or assistant attribution.
# Install: cp scripts/commit-msg-hook.sh .git/hooks/commit-msg && chmod +x .git/hooks/commit-msg
msg_file="$1"
if grep -qiE 'co-authored-by|claude|anthropic|generated with' "$msg_file"; then
  echo "commit-msg hook: message contains a forbidden attribution word (co-authored-by, claude, anthropic, generated with). Commit rejected." >&2
  exit 1
fi
exit 0
