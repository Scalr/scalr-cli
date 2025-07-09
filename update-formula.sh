#!/bin/bash

# Script to update Homebrew formula for new releases
# Usage: ./update-formula.sh <version>

set -e

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 0.17.2"
    exit 1
fi

VERSION=$1
FORMULA_FILE="Formula/scalr-cli.rb"

# Validate version format
if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format X.Y.Z (e.g., 0.17.2)"
    exit 1
fi

echo "Updating formula for version $VERSION..."

# Download the tarball and calculate SHA256
TARBALL_URL="https://github.com/Scalr/scalr-cli/archive/refs/tags/v${VERSION}.tar.gz"
TEMP_FILE=$(mktemp)

echo "Downloading tarball: $TARBALL_URL"
if ! curl -sL "$TARBALL_URL" -o "$TEMP_FILE"; then
    echo "Error: Failed to download tarball. Make sure the release v${VERSION} exists."
    rm -f "$TEMP_FILE"
    exit 1
fi

# Calculate SHA256
SHA256=$(shasum -a 256 "$TEMP_FILE" | cut -d' ' -f1)
rm -f "$TEMP_FILE"

echo "SHA256: $SHA256"

# Update the formula file
sed -i.bak \
    -e "s|url \".*\"|url \"https://github.com/Scalr/scalr-cli/archive/refs/tags/v${VERSION}.tar.gz\"|" \
    -e "s|sha256 \".*\"|sha256 \"${SHA256}\"|" \
    "$FORMULA_FILE"

echo "Formula updated successfully!"
echo "Changes made to $FORMULA_FILE:"
echo "- Version: v${VERSION}"
echo "- SHA256: ${SHA256}"

# Show the diff
if command -v diff >/dev/null 2>&1; then
    echo ""
    echo "Diff:"
    diff "${FORMULA_FILE}.bak" "$FORMULA_FILE" || true
fi

echo ""
echo "Next steps:"
echo "1. Test the formula: brew install --build-from-source ./Formula/scalr-cli.rb"
echo "2. Run tests: brew test scalr-cli"
echo "3. Commit the changes: git add Formula/scalr-cli.rb && git commit -m 'Update formula to v${VERSION}'"
echo "4. Push to tap repository or submit to Homebrew Core" 