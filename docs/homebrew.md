# Homebrew release

After a `v*` tag is released, the `atellar` CLI is published to Homebrew
automatically. The release workflow runs GoReleaser (`.goreleaser.yaml`), which
attaches `atellar` archives for darwin/linux (amd64/arm64) to the GitHub Release
and pushes an updated formula to the tap repository.

## One-time setup

1. Create a public repository named `hasirciogluhq/homebrew-tap` with at least
   one commit (for example a README). Homebrew requires the `homebrew-` prefix.
2. Create a fine-grained personal access token with `Contents: Read and write`
   on `hasirciogluhq/homebrew-tap` only.
3. Add it to this repository as an Actions secret named
   `HOMEBREW_TAP_GITHUB_TOKEN` (Settings, Secrets and variables, Actions).
   `GITHUB_TOKEN` is provided automatically and needs no setup.

If `homebrew-tap` already exists for another project, the formula
`Formula/atellar.rb` is added next to the existing ones.

## Install

```bash
brew tap hasirciogluhq/tap
brew install atellar
```

## Notes

- The linux tarball, `install.sh` and the release itself are still produced by
  `scripts/release/*`; GoReleaser only appends the CLI archives.
- Test a config locally without publishing:
  `goreleaser release --snapshot --clean --skip=publish`.
