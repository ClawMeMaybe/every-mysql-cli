#!/bin/bash
set -euo pipefail
REPO_DIR="/home/agent/every-mysql-cli"
cd "$REPO_DIR"
git fetch origin --quiet 2>/dev/null
SPEC_BRANCHES=$(git branch -r | grep -E 'origin/(feature|spec)/' | sed 's/^[[:space:]]*//' || true)
if [ -z "$SPEC_BRANCHES" ]; then
    exit 0
fi
while IFS= read -r branch; do
    BRANCH_NAME=$(echo "$branch" | sed 's|^origin/||')
    OPEN_PR=$(gh pr list --head "$BRANCH_NAME" --state open --json number 2>/dev/null | grep -c 'number' || true)
    if [ "$OPEN_PR" -gt 0 ]; then
        continue
    fi
    git checkout "$branch" --quiet 2>/dev/null || continue
    SPEC_FILE=$(find docs/superpowers/spec docs/superpowers/specs -name '*.md' 2>/dev/null | head -1 || true)
    git checkout - --quiet 2>/dev/null || true
    if [ -z "$SPEC_FILE" ]; then
        continue
    fi
    ISSUE_NUM=$(gh issue list --limit 100 --json number,body --state open 2>/dev/null | grep -B1 "$BRANCH_NAME" | grep -oP '"number":\K[0-9]+' | head -1 || true)
    if [ -z "$ISSUE_NUM" ]; then
        continue
    fi
    HAS_PENDING=$(gh issue view "$ISSUE_NUM" --json labels --jq '.labels[].name' 2>/dev/null | grep -c 'pending' || true)
    if [ "$HAS_PENDING" -eq 0 ]; then
        continue
    fi
    gh issue edit "$ISSUE_NUM" --remove-label pending --add-label in_progress 2>/dev/null || true
    echo "Found spec to process:"
    echo "  Branch: $BRANCH_NAME"
    echo "  Issue: #$ISSUE_NUM"
    echo "  Spec: $SPEC_FILE"
    echo "PROCESS:$BRANCH_NAME:$ISSUE_NUM:$SPEC_FILE"
    exit 0
done <<< "$SPEC_BRANCHES"
