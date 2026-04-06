#!/bin/bash
# Update ServerDash version script

set -e

# Move to project root
cd "$(dirname "$0")/.."

# Validate input version
if [ -z "$1" ]; then
    echo "Error: please provide a new version number."
    echo "Usage: $0 <new_version>"
    echo "Example: $0 0.5.1"
    exit 1
fi

NEW_VERSION=$1

# Validate semantic version format
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    echo "Error: invalid version format. Use semver like 1.2.3 or 1.2.3-rc1."
    exit 1
fi

OLD_VERSION=$(cat VERSION | tr -d '[:space:]')

echo "Current version: $OLD_VERSION"
echo "Target version: $NEW_VERSION"
echo "Updating files..."

# 1. Update /VERSION
echo "$NEW_VERSION" > VERSION
echo "✅ Updated VERSION file"

# 2. Update web/package.json and web/package-lock.json
cd web
npm version "$NEW_VERSION" --no-git-tag-version --allow-same-version
cd ..
echo "✅ Updated web/package.json and package-lock.json"

# 3. Update install.sh fallback version
sed -i "s/|| echo \"[0-9][0-9.]*[0-9]\")/|| echo \"$NEW_VERSION\")/g" install.sh
sed -i "s/SERVERDASH_VERSION=\"[0-9][0-9.]*[0-9]\"/SERVERDASH_VERSION=\"$NEW_VERSION\"/g" install.sh
echo "✅ Updated install.sh fallback version"

# 4. Update version record in memory.md
sed -i "s/- \*\*Version\*\*: see \/VERSION \(single source of truth\)/- **Version**: see \/VERSION (single source of truth)/g" memory.md
echo "✅ Verified memory.md version record"

echo "--------------------------------------------------"
echo "🎉 Version updated to $NEW_VERSION"
echo "Tip: remember to update changelog.md manually."
echo "If you are ready to commit, run:"
echo ""
echo "git add VERSION web/package.json web/package-lock.json install.sh memory.md"
echo "git commit -m \"chore: bump version to $NEW_VERSION\""
echo "git tag v$NEW_VERSION"
echo "git push origin main --tags"
