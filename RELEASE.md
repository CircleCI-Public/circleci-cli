# Release Process

Releases are made by merging the release PR. The CLI is published to
[GitHub releases](https://github.com/CircleCI-Public/circleci-cli/releases) and every package
manager goreleaser targets, plus packagecloud, Cloudsmith and Docker Hub.

## How it works

On every push to main, after `check`, `test` and `docs` pass:

- **The `release-pr` job** runs `cmd/ci/release` (`task ci:release-pr`). Once the newest
  version in `CHANGELOG.md` is tagged, it opens or updates the "Release vX.Y.Z" PR from the
  `release/next` branch, which adds a section for the next version to `CHANGELOG.md` with
  GitHub's notes on the PRs merged since the last release. With none merged, the PR is closed.
- **The `deploy` job** halts unless the newest version in `CHANGELOG.md` isn't tagged yet — that
  is, unless this push merged its release PR (`task ci:halt-unless-unreleased`). Then it tags the
  version, runs goreleaser with the version's section as the GitHub release notes, publishes the
  Linux packages, and tags `clikit/vX.Y.Z` (`task ci:release`).

Each changelog entry is a merged PR's title, so write titles the way the commit subjects are
written: plain and imperative.

## The version

The version is the newest `## [x.y.z]` heading in `CHANGELOG.md`; there's no other version
file. Releases continue from `v1.0.52041`, the last of the releases numbered by CI build.

A release is a minor bump (1.0.52041 → 1.1.0), unless a PR in it has one of these labels:

- `release:major`: a major bump (2.0.0)
- `release:patch`: a patch bump (1.1.1), if every PR in the release has it

Label the PR before it's merged, or relabel it and rerun the latest main build's `release-pr`
job.

Don't edit the release PR's branch: it's rewritten on every push to main.

## Credentials

| Context | Variable | For |
|---|---|---|
| `devex-release` | `GITHUB_TOKEN` | The release PR, the tags and the GitHub release |
| `devex-release` | the package manager and Docker Hub credentials | Publishing |
| `macos-codesigning` | the signing and notarization credentials | Signing the macOS binaries |

## If a release fails part way

The `deploy` job pushes the tag before it publishes anything, and a rerun halts once the tag
exists. If it failed before goreleaser published the GitHub release, delete the tag
(`git push --delete origin vX.Y.Z`) and rerun the job. Otherwise finish by hand whatever steps
of `task ci:release` didn't run, from the job's output.
