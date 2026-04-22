#!/bin/bash
# poll-specs.sh - 扫描远程分支，找到有待处理 spec 的分支

REPO_DIR="/home/agent/every-mysql-cli"
cd "$REPO_DIR" || exit 1

# Fetch 所有远程分支
git fetch --all --prune 2>/dev/null

# 获取所有远程 feature/ 和 spec/ 分支
BRANCHES=$(git branch -r | grep -E "origin/(feature|spec)/" | sed "s/.*origin\/\(.*\)/\1/")

if [ -z "$BRANCHES" ]; then
  echo "No feature/ or spec/ branches found"
  exit 0
fi

for branch in $BRANCHES; do
  # 检查该分支是否有 spec 文件
  SPECS=$(git ls-tree -r --name-only origin/$branch | grep -E "docs/superpowers/specs?/.*\.md$" || true)
  
  if [ -z "$SPECS" ]; then
    continue
  fi
  
  # 检查是否已有 open 的 PR
  HAS_PR=$(gh pr list --head "$branch" --state open --json number --jq "length" 2>/dev/null || echo "0")
  
  if [ "$HAS_PR" -gt 0 ]; then
    echo "PR already exists for $branch, skipping"
    continue
  fi
  
  # 查找关联的 tracking issue（包含分支名的 spec issue）
  MATCHED_ISSUE=$(gh issue list --label "spec" --state open --limit 50 --json number,title,body --jq ".[] | select(.body | contains(\"$branch\")) | .number" 2>/dev/null | head -1)
  
  if [ -z "$MATCHED_ISSUE" ]; then
    echo "No tracking issue found for $branch"
    continue
  fi
  
  ISSUE_NUMBER="$MATCHED_ISSUE"
  
  # 从 issue body 中提取 spec 文件路径
  ISSUE_BODY=$(gh issue view "$ISSUE_NUMBER" --json body --jq ".body")
  SPEC_FILE=$(echo "$ISSUE_BODY" | grep -i "Spec File" | head -1 | rev | cut -d'`' -f2 | rev)
  
  if [ -z "$SPEC_FILE" ]; then
    SPEC_FILE=$(echo "$SPECS" | head -1)
  fi
  
  # 抢占：将 label 从 pending 改为 in_progress
  CURRENT_LABELS=$(gh issue view "$ISSUE_NUMBER" --json labels --jq ".[].name" 2>/dev/null)
  
  if echo "$CURRENT_LABELS" | grep -q "pending"; then
    gh issue edit "$ISSUE_NUMBER" --remove-label "pending" --add-label "in_progress" 2>/dev/null
  fi
  
  echo "Found spec to process:"
  echo "  Branch: $branch"
  echo "  Issue: #$ISSUE_NUMBER"
  echo "  Spec: $SPEC_FILE"
  echo "PROCESS:$branch:$ISSUE_NUMBER:$SPEC_FILE"
  exit 0
done

echo "No pending specs found"
