#!/bin/bash

set -x

# Move to the repository root
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd $REPO_ROOT

# Enable custom git hooks in .githooks
make git-hooks-setup

# Install goreleaser for release automation
echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | sudo tee /etc/apt/sources.list.d/goreleaser.list
sudo apt update
sudo apt install -y goreleaser
