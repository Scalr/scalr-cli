# Release Checklist for Scalr CLI

## Pre-Release Preparation

### 1. Code Review & Testing
- [ ] All changes reviewed and approved
- [ ] Local tests pass: `make test`
- [ ] Local build works: `make build`
- [ ] Version system works: `./scalr -version`
- [ ] Multi-platform build works: `./build-all.sh`

### 2. Documentation
- [ ] README updated with new features
- [ ] CHANGELOG.md updated with version notes
- [ ] API documentation updated (if applicable)
- [ ] Breaking changes documented

### 3. Version Preparation
- [ ] Decide on version number (semantic versioning)
- [ ] Update any hardcoded version references
- [ ] Ensure version embedding works correctly

## Release Process

### 1. Create Release
```bash
# Ensure you're on main branch
git checkout main
git pull origin main

# Create and push tag
git tag v0.17.2
git push origin v0.17.2

# Or use GitHub Actions
# Go to GitHub Actions -> Release workflow -> Run workflow
# Input version: 0.17.2
```

### 2. Verify Release
- [ ] GitHub release created successfully
- [ ] All platform binaries generated
- [ ] SHA256 checksums available
- [ ] Release notes are correct

### 3. Update Homebrew Formula
```bash
# Use the update script
./update-formula.sh 0.17.2

# Or manually update:
# 1. Update URL in Formula/scalr-cli.rb
# 2. Update SHA256 hash
# 3. Test the formula
```

### 4. Test Homebrew Formula
```bash
# Test installation
brew uninstall scalr-cli || true
brew install --build-from-source ./Formula/scalr-cli.rb

# Test functionality
scalr -version
scalr -help

# Run Homebrew tests
brew test scalr-cli

# Check style
brew style scalr-cli
```

## Post-Release

### 1. Homebrew Distribution

#### Option A: Homebrew Core Submission
```bash
# Fork homebrew-core
gh repo fork Homebrew/homebrew-core

# Clone and create branch
git clone https://github.com/YOUR_USERNAME/homebrew-core.git
cd homebrew-core
git checkout -b scalr-cli-0.17.2

# Copy formula
cp ../scalr-cli/Formula/scalr-cli.rb Formula/scalr-cli.rb

# Test and submit
brew install --build-from-source ./Formula/scalr-cli.rb
brew test scalr-cli
brew audit --new scalr-cli

# Create PR
git add Formula/scalr-cli.rb
git commit -m "scalr-cli 0.17.2 (new formula)"
git push origin scalr-cli-0.17.2
gh pr create --title "scalr-cli 0.17.2 (new formula)"
```

#### Option B: Homebrew Tap
```bash
# Create/update homebrew-scalr repository
# Copy Formula/scalr-cli.rb to that repository
# Users install with: brew tap Scalr/scalr && brew install scalr-cli
```

### 2. Announcements
- [ ] Update company documentation
- [ ] Announce in relevant channels
- [ ] Update package managers (if applicable)

### 3. Monitor
- [ ] Check for installation issues
- [ ] Monitor GitHub issues
- [ ] Respond to user feedback

## Version-Specific Notes

### v0.17.2 (Current)
- Enhanced version system with git commit and build date
- Dynamic version embedding for Homebrew
- Improved build process
- Homebrew Core ready

## Rollback Plan

If issues are discovered:

1. **Immediate**: Remove from Homebrew (if published)
2. **GitHub**: Create hotfix release or revert tag
3. **Communication**: Notify users of issues
4. **Fix**: Address problems and create new release

## Automation Opportunities

Future improvements:
- [ ] Automated formula updates via GitHub Actions
- [ ] Automated testing on multiple platforms
- [ ] Automated Homebrew Core PR creation
- [ ] Integration with package managers

## Resources

- [Semantic Versioning](https://semver.org/)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github)
- [Scalr CLI Documentation](https://github.com/Scalr/scalr-cli) 