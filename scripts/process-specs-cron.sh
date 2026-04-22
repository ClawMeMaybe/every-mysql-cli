#!/bin/bash
# process-specs-cron.sh - Full spec-driven pipeline cron script

set -euo pipefail

REPO_DIR="/home/agent/every-mysql-cli"
POLL_SCRIPT="$REPO_DIR/scripts/poll-specs.sh"

cd "$REPO_DIR"

# Step 1: Poll for pending specs
echo "=== Starting spec pipeline ==="
PROCESS_OUTPUT=$($POLL_SCRIPT 2>&1 | grep "^PROCESS:" || true)

if [ -z "$PROCESS_OUTPUT" ]; then
    echo "No pending specs found."
    exit 0
fi

echo "Found: $PROCESS_OUTPUT"

# Step 2: Parse PROCESS output: PROCESS:<branch>:<issue_number>:<spec_file>
BRANCH=$(echo "$PROCESS_OUTPUT" | cut -d: -f2)
ISSUE_NUM=$(echo "$PROCESS_OUTPUT" | cut -d: -f3)
SPEC_FILE=$(echo "$PROCESS_OUTPUT" | cut -d: -f4-)

echo "Branch: $BRANCH"
echo "Issue: #$ISSUE_NUM"
echo "Spec: $SPEC_FILE"

# Step 3: Checkout the branch
echo "Checking out branch: $BRANCH"
git fetch origin "$BRANCH"
git checkout "$BRANCH"

# Step 4: Read spec content
echo "Spec content length: $(wc -c < "$SPEC_FILE") bytes"

# Step 5: Run Claude Code to implement the spec
echo "Running Claude Code implementation..."
cat > /tmp/claude-prompt.txt << ENDOFPROMPT
You are implementing a spec from the spec-driven development pipeline.

IMPORTANT RULES:
1. First read the spec file: $SPEC_FILE
2. Implement ALL requirements in the spec completely
3. Make commits frequently with clear descriptive commit messages
4. DO NOT modify the spec file itself
5. After implementation ensure all changes are committed
6. Use conventional commits format feat: fix: docs: etc.

Please implement this spec now. Read the spec file, plan your approach, and implement all requirements.
ENDOFPROMPT

claude --dangerously-skip-permissions --print --max-turns 100 -p "$(cat /tmp/claude-prompt.txt)" 2>&1 || true

# Step 6: Commit any remaining changes
echo "Checking for uncommitted changes..."
if ! git diff --quiet || [ -n "$(git ls-files --others --exclude-standard)" ]; then
    echo "Committing remaining changes..."
    git add -A
    git commit -m "feat: implement spec from issue #$ISSUE_NUM"
fi

# Step 7: Push changes
echo "Pushing to origin/$BRANCH..."
git push origin "$BRANCH" 2>&1 || git push --force-with-lease origin "$BRANCH" 2>&1 || true

# Step 8: Create PR
echo "Creating pull request..."
PR_URL=$(gh pr create --base main --head "$BRANCH" --title "Implement spec #$ISSUE_NUM" --body "Auto-generated PR from spec-driven pipeline.

Spec: \`$SPEC_FILE\`
Issue: #$ISSUE_NUM
Branch: \`$BRANCH\`

This PR implements the spec automatically via Claude Code + oh-my-claudecode.

Please review and merge when ready." 2>&1) || true
echo "PR created: $PR_URL"

# Extract PR number from output
PR_NUM=$(echo "$PR_URL" | grep -oP "/pull/\K[0-9]+" || echo "")

if [ -n "$PR_NUM" ]; then
    echo "PR #$PR_NUM created"
    sleep 5
    MERGEABLE=$(gh pr view "$PR_NUM" --json mergeable --jq ".mergeable" 2>/dev/null || echo "UNKNOWN")

    if [ "$MERGEABLE" = "CONFLICTING" ]; then
        echo "PR has conflicts attempting resolution..."
        git fetch origin main
        if git rebase origin/main 2>&1; then
            git push --force-with-lease origin "$BRANCH"
            echo "Resolved via rebase"
        else
            git rebase --abort 2>/dev/null || true
            if git merge origin/main 2>&1; then
                git push origin "$BRANCH"
                echo "Resolved via merge"
            else
                git merge --abort 2>/dev/null || true
                echo "Conflict resolution failed"
            fi
        fi
    fi

    gh issue comment "$ISSUE_NUM" --body "PR created: $PR_URL" 2>&1 || true
    gh issue edit "$ISSUE_NUM" --remove-label "in_progress" --add-label "pr-created" 2>&1 || true
else
    EXISTING_PR=$(gh pr list --head "$BRANCH" --state open --json url --jq ".[0].url" 2>/dev/null || echo "")
    if [ -n "$EXISTING_PR" ]; then
        gh issue edit "$ISSUE_NUM" --remove-label "in_progress" --add-label "pr-created" 2>&1 || true
    else
        gh issue edit "$ISSUE_NUM" --remove-label "in_progress" --add-label "failed" 2>&1 || true
    fi
fi

echo "=== Pipeline finished ==="
