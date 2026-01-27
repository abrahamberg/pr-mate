#!/bin/bash

if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GITHUB_TOKEN environment variable is required"
    exit 1
fi

echo "Authenticating with GitHub..."
export GH_TOKEN="$GITHUB_TOKEN"

# Authenticate gh CLI
echo "$GITHUB_TOKEN" | gh auth login --with-token 2>/dev/null || {
    echo "Note: gh authentication uses GH_TOKEN environment variable"
}

# Verify authentication
if gh auth status 2>&1 | grep -q "Logged in"; then
    echo "✓ GitHub CLI authenticated successfully"
else
    echo "⚠ GitHub CLI authentication status:"
    gh auth status 2>&1 || true
fi

# Verify copilot is accessible
if command -v copilot &> /dev/null; then
    echo "✓ Copilot CLI ready: $(copilot --version 2>&1 | head -1)"
else
    echo "⚠ Warning: Copilot CLI not found in PATH"
fi

echo "Starting application..."

# Run the application
exec "$@"
