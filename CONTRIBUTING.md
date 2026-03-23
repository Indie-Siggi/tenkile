# Contributing to Tenkile

Thank you for your interest in contributing to Tenkile! This guide covers how to contribute device profiles, code, and documentation.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Contributing Device Profiles](#contributing-device-profiles)
- [Contributing Code](#contributing-code)
- [Contributing Documentation](#contributing-documentation)
- [Submitting Changes](#submitting-changes)

## Code of Conduct

This project follows a **be excellent to each other** policy. All contributors are expected to be respectful and constructive.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/tenkile.git
   cd tenkile
   ```
3. **Install dependencies**:
   ```bash
   go mod download
   ```
4. **Run tests**:
   ```bash
   make test
   ```
5. **Build**:
   ```bash
   make build
   ```

## Contributing Device Profiles

Device profiles are the backbone of Tenkile's curated database. Your contributions help other users with similar devices get optimal playback without manual configuration.

### Why Contribute Device Profiles?

- Smart TVs have fixed capabilities per model
- Browser-based probing is unreliable on Smart TVs
- A profile you submit helps everyone with the same device
- Community voting ensures quality

### Profile Quality Requirements

A good device profile includes:

1. **Accurate device identification**
   - Full model number (e.g., "Samsung QN65Q80TAFXZA")
   - Manufacturer name
   - Platform (samsung_tizen, lg_webos, roku, android_tv)

2. **Complete codec capabilities**
   - All supported video codecs
   - All supported audio codecs
   - Maximum resolution and framerate
   - HDR support (HDR10, Dolby Vision, HLG)
   - DRM systems (Widevine, PlayReady, FairPlay)

3. **Known issues** (if any)
   - Specific codec/container combinations that don't work
   - Firmware limitations
   - Workarounds if known

4. **Testing evidence**
   - Test files used
   - Firmware version tested
   - Date of testing

### Profile Format

```json
{
    "id": "samsung-qn65q80tafxza-2024",
    "name": "Samsung QN65Q80TAFXZA",
    "manufacturer": "Samsung",
    "model": "QN65Q80TAFXZA",
    "platform": "samsung_tizen",
    "os_versions": ["1005.4", "1006.1"],
    "capabilities": {
        "platform": "samsung_tizen",
        "video_codecs": ["hevc", "av1", "vp9", "avc"],
        "audio_codecs": ["aac", "ac3", "eac3", "flac", "opus", "dts"],
        "containers": ["mp4", "mkv", "webm", "mov", "ts"],
        "max_width": 3840,
        "max_height": 2160,
        "max_framerate": 120,
        "supports_hdr": true,
        "supports_dolby_vision": false,
        "supports_hdr10_plus": true,
        "supports_hlg": true,
        "drm_systems": ["widevine", "playready"]
    },
    "known_issues": [
        {
            "title": "Dolby Vision not supported",
            "description": "This model does not support Dolby Vision playback",
            "severity": "warning",
            "workaround": "Transcode to HDR10"
        }
    ],
    "source": "community",
    "notes": "Tested with firmware 1005.4 on March 2024",
    "created_at": "2024-03-15T10:00:00Z",
    "last_updated": "2024-03-15T10:00:00Z"
}
```

### How to Submit a Profile

#### Option 1: GitHub Pull Request (Recommended)

1. Fork the repository
2. Add your profile to `data/curated/`:
   - Use the naming convention: `{manufacturer}_{model}.json`
   - Example: `samsung_qn65q80tafxza.json`
3. Include comprehensive test results in the `notes` field
4. Submit a pull request with:
   - Clear title: "Add device profile for [Device Name]"
   - Description of testing performed
   - Any known limitations

#### Option 2: Community Forum

Post your device profile on the Tenkile community forum with:
- Device model number
- Complete JSON profile
- Test results
- Your testing methodology

### Testing Your Profile

Before submitting, verify your profile works:

1. **Start Tenkile** with your profile:
   ```bash
   tenkile --curated-path ./data/curated
   ```

2. **Check the curated database stats**:
   ```bash
   curl http://localhost:8096/api/v1/admin/stats/database
   ```

3. **Test playback** with your device and verify:
   - Correct codec is selected
   - No unnecessary transcoding
   - HDR is handled correctly

4. **Check decision logs**:
   ```bash
   curl http://localhost:8096/api/v1/admin/decisions?limit=5
   ```

### Profile Validation

Your profile will be validated for:

- Valid JSON syntax
- Required fields present
- Codec values are recognized
- Resolution/framerate values are reasonable
- No conflicting capabilities

### Voting and Verification

After submission:

1. **Community voting**: Other users vote on accuracy
2. **Verification**: Maintainers verify high-voted profiles
3. **Promotion**: Verified profiles get `verified: true`
4. **Bundling**: High-quality profiles may be added to embedded bundles

## Contributing Code

### Development Workflow

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** following the style guide

3. **Add tests** for new functionality

4. **Run the full test suite**:
   ```bash
   make test
   make lint
   ```

5. **Build** to ensure compilation succeeds:
   ```bash
   make build
   ```

### Code Style

- Use `gofmt` for formatting
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Add SPDX license headers to new files
- Keep functions focused and small
- Document exported functions

### Commit Messages

Follow conventional commits:

```
feat: add HDR tone mapping for non-HDR devices
fix: correct FFmpeg encoder selection for VP9
docs: update API documentation for Phase 3
test: add test cases for trust adjustment logic
```

### Pull Request Guidelines

- Reference issues in your PR description
- Keep PRs focused and small
- Include tests
- Update documentation if needed
- Be responsive to review feedback

## Contributing Documentation

Documentation improvements are always welcome:

- Fix typos or unclear explanations
- Add examples to API docs
- Improve inline code comments
- Write tutorials or how-tos
- Translate documentation

### Documentation Files

| File | Purpose |
|------|---------|
| `README.md` | Project overview |
| `AGENTS.md` | Agent task guide |
| `docs/PHASE*.md` | Phase-specific documentation |
| `docs/ARCHITECTURE.md` | System architecture |
| `docs/CODEC_REFERENCE.md` | Codec capabilities |
| `docs/DEVICE_DETECTION.md` | Device detection |

## Submitting Changes

### Pull Request Checklist

- [ ] Code follows style guidelines
- [ ] Tests pass locally
- [ ] Documentation updated (if applicable)
- [ ] Commits are logically organized
- [ ] PR description explains the change

### Review Process

1. Automated checks run (build, test, lint)
2. Maintainers review for:
   - Code quality
   - Design alignment
   - Documentation updates
   - Test coverage
3. Changes may be requested
4. Once approved, maintainers merge

## License

By contributing to Tenkile, you agree that your contributions will be licensed under the [AGPL-3.0-or-later license](../LICENSE).

## Questions?

- Open an issue for bugs or feature requests
- Check existing issues before duplicating
- Join the community forum for discussions
