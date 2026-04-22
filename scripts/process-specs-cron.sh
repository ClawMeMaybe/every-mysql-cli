#!/bin/bash
# process-specs-cron.sh - Full spec-driven pipeline with ralph mode

set -euo pipefail

REPO_DIR="/home/agent/every-mysql-cli"
POLL_SCRIPT="$REPO_DIR/scripts/poll-specs.sh"
LOG_FILE="/home/agent/pipeline.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

notify() {
    local msg="$1"
    log "NOTIFY: $msg"
    echo "$msg" >> /home/agent/pipeline-status/STATUS.md
}

cd "$REPO_DIR"

log "=== Starting spec pipeline ==="
PROCESS_OUTPUT=$($POLL_SCRIPT 2>&1 | grep '^PROCESS:' || true)

if [ -z "$PROCESS_OUTPUT" ]; then
    log "No pending specs found."
    exit 0
fi

log "Found: $PROCESS_OUTPUT"

BRANCH=$(echo "$PROCESS_OUTPUT" | cut -d: -f2)
ISSUE_NUM=$(echo "$PROCESS_OUTPUT" | cut -d: -f3)
SPEC_FILE=$(echo "$PROCESS_OUTPUT" | cut -d: -f4-)

log "Branch: $BRANCH"
log "Issue: #$ISSUE_NUM"
log "Spec: $SPEC_FILE"

cat > /home/agent/pipeline-status/STATUS.md << EOF
# Pipeline Status

**Spec**: $SPEC_FILE
**Branch**: $BRANCH
**Issue**: #$ISSUE_NUM
**Status**: Starting ralph mode implementation
**Started**: $(date '+%Y-%m-%d %H:%M:%S')
EOF

notify "Spec pipeline started: Issue #$ISSUE_NUM on branch $BRANCH"

log "Checking out branch: $BRANCH"
git fetch origin "$BRANCH"
git checkout "$BRANCH"

SPEC_SIZE=$(wc -c < "$SPEC_FILE")
log "Spec content length: $SPEC_SIZE bytes"
notify "Checked out $BRANCH. Spec has $SPEC_SIZE bytes. Starting ralph implementation..."

log "Running Claude Code ralph mode..."
claude --dangerously-skip-permissions --print --max-turns 100 -p "ralph. Read and fully implement $SPEC_FILE. Implement ALL requirements. Commit frequently with conventional commits. Do NOT modify the spec file. Write tests. Keep going until fully implemented - the boulder never stops." 2>&1 | tee -a "$LOG_FILE" || true

COMMITS_MADE=$(git log --oneline origin/$BRANCH..HEAD 2>/dev/null | wc -l | tr -d ' ')
log "Claude Code done. Commits: $COMMITS_MADE"
notify "Ralph implementation complete. Commits: $COMMITS_MADE. Pushing and creating PR..."

if ! git diff --quiet || [ -n "$(git ls-files --others --exclude-standard)" ]; then
    log "Committing remaining changes..."
    git add -A
    git commit -m "feat: implement spec from issue #$ISSUE_NUM"
fi

log "Pushing to origin/$BRANCH..."
git push origin "$BRANCH" 2>&1 || git push --force-with-lease origin "$BRANCH" 2>&1 || true

log "Creating PR..."
PR_URL=$(gh pr create --base main --head "$BRANCH" --title "Implement spec #$ISSUE_NUM (ralph)" --body "Auto-generated PR via Claude Code ralph mode.

Spec: \`$SPEC_FILE\`
Issue: #$ISSUE_NUM
Branch: \`$BRANCH\`" 2>&1) || true
log "PR: $PR_URL"

PR_NUM=$(echo "$PR_URL" | grep -oP '/pull/\K[0-9]+' || echo "")

if [ -n "$PR_NUM" ]; then
    sleep 5
    MERGEABLE=$(gh pr view "$PR_NUM" --json mergeable --jq '.mergeable' 2>/dev/null || echo "UNKNOWN")
    if [ "$MERGEABLE" = "CONFLICTING" ]; then
        log "Conflicts detected, resolving..."
        git fetch origin main
        if git rebase origin/main 2>&1; then
            git push --force-with-lease origin "$BRANCH"
        else
            git rebase --abort 2>/dev/null || true
            git merge origin/main 2>&1 && git push origin "$BRANCH" || git merge --abort 2>/dev/null || true
        fi
    fi
    gh issue comment "$ISSUE_NUM" --body "PR created: $PR_URL" 2>&1 || true
    gh issue edit "$ISSUE_NUM" --remove-label in_progress --add-label pr-created 2>&1 || true
    cat > /home/agent/pipeline-status/STATUS.md << EOF
# Pipeline Status - COMPLETE

**Spec**: $SPEC_FILE
**Issue**: #$ISSUE_NUM
**PR**: $PR_URL
**Commits**: $COMMITS_MADE
**Completed**: $(date '+%Y-%m-%d %H:%M:%S')
EOF
    notify "Pipeline complete! PR #$PR_NUM: $PR_URL"
else
    EXISTING_PR=$(gh pr list --head "$BRANCH" --state open --json url --jq '.[0].url' 2>/dev/null || echo "")
    if [ -n "$EXISTING_PR" ]; then
        gh issue edit "$ISSUE_NUM" --remove-label in_progress --add-label pr-created 2>&1 || true
        notify "Pipeline done. Existing PR: $EXISTING_PR"
    else
        gh issue edit "$ISSUE_NUM" --remove-label in_progress --add-label failed 2>&1 || true
        notify "Pipeline FAILED for Issue #$ISSUE_NUM"
    fi
fi

log "=== Pipeline finished ==="

# NOTE: This function is appended to the existing script
# WeChat notification via OpenClaw gateway API
send_wechat() {
    local msg="$1"
    # Use openclaw CLI to broadcast to weixin
    openclaw message broadcast --channel openclaw-weixin --message "$msg" --dry-run 2>/dev/null || true
    # If dry-run works, try actual send
    openclaw message broadcast --channel openclaw-weixin --message "$msg" 2>&1 >> "$LOG_FILE" || true
}
