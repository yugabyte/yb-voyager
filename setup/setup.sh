#!/usr/bin/env bash

REPO_ROOT="$(git rev-parse --show-toplevel)"
SOURCE_HOOK_DIR="$REPO_ROOT/setup/hooks"
TARGET_HOOK_DIR="$REPO_ROOT/.git/hooks"

# Copy hooks and make them executable
echo "Installing git hooks..."
for hook in $(ls $SOURCE_HOOK_DIR); do
    cp "$SOURCE_HOOK_DIR/$hook" "$TARGET_HOOK_DIR/$hook"
    chmod +x "$TARGET_HOOK_DIR/$hook"
    echo "Installed $hook"
done

echo "Git hooks setup completed successfully."
