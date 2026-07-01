#!/bin/bash
set -e

UPSTREAM_URL="git@github.com:QuantumNous/new-api.git"
UPSTREAM_NAME="upstream"
TARGET_BRANCH="feature/merge-upstream"

echo "=== Merge Upstream Script ==="
echo "Upstream: $UPSTREAM_URL"
echo "Target branch: $TARGET_BRANCH"
echo ""

# 1. Ensure we're on the target branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "$TARGET_BRANCH" ]; then
  echo "[1/5] Switching to $TARGET_BRANCH..."
  git checkout "$TARGET_BRANCH"
else
  echo "[1/5] Already on $TARGET_BRANCH"
fi

# 2. Add upstream remote if not exists
if git remote get-url "$UPSTREAM_NAME" &>/dev/null; then
  echo "[2/5] Upstream remote already exists"
  CURRENT_URL=$(git remote get-url "$UPSTREAM_NAME")
  if [ "$CURRENT_URL" != "$UPSTREAM_URL" ]; then
    echo "  WARNING: Upstream URL differs. Current: $CURRENT_URL"
    echo "  Updating upstream URL..."
    git remote set-url "$UPSTREAM_NAME" "$UPSTREAM_URL"
  fi
else
  echo "[2/5] Adding upstream remote..."
  git remote add "$UPSTREAM_NAME" "$UPSTREAM_URL"
fi

# 3. Fetch upstream
echo "[3/5] Fetching upstream..."
git fetch "$UPSTREAM_NAME"

# 4. Stash local changes if any
STASHED=false
if ! git diff-index --quiet HEAD --; then
  echo "[4/5] Stashing local changes..."
  git stash push -m "merge-upstream: auto stash before merge"
  STASHED=true
else
  echo "[4/5] Working tree is clean, no need to stash"
fi

# 5. Merge upstream main
echo "[5/5] Merging upstream/main into $TARGET_BRANCH..."
if git merge "$UPSTREAM_NAME/main" --no-edit; then
  echo ""
  echo "=== Merge completed successfully ==="
else
  echo ""
  echo "=== Merge conflicts detected ==="
  echo "Conflicting files:"
  git diff --name-only --diff-filter=U
  echo ""
  echo "Please resolve the conflicts manually, then run:"
  echo "  git add ."
  echo "  git commit -m 'merge: upstream main'"
  echo ""
  echo "To abort the merge:"
  echo "  git merge --abort"
fi

# Restore stash if needed
if $STASHED; then
  echo ""
  echo "NOTE: Local changes were stashed before merge."
  echo "  To restore: git stash pop"
fi
