# Changelog

All notable changes to the CircleCI CLI will be documented in this file. Each release's section
is added by its release PR (see [RELEASE.md](RELEASE.md)) and becomes its GitHub release notes.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.5.0] - 2026-10-09

### What's Changed
* Sign the orb init commit when commit.gpgSign is set by @conormcd in https://github.com/CircleCI-Public/circleci-cli/pull/1892
* Stop artifact downloads timing out after 30 seconds by @pete-woods in https://github.com/CircleCI-Public/circleci-cli/pull/1903
* Bump github.com/go-git/go-git/v6 in / by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1898
* Bump github.com/segmentio/analytics-go/v3 in / by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1899
* Bump charm.land/bubbletea/v2 in / by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1864
* Bump github.com/NimbleMarkets/ntcharts/v2 in /clikit by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1865
* Fix main after segment client upgrade by @pete-woods in https://github.com/CircleCI-Public/circleci-cli/pull/1904


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.4.0...v1.5.0

## [1.4.0] - 2026-10-08

### What's Changed
* Name the upgrade command in the update notice by @EnoshAnwar in https://github.com/CircleCI-Public/circleci-cli/pull/1891
* Add circleci runner fleet list/get by @soulchips in https://github.com/CircleCI-Public/circleci-cli/pull/1893


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.3.0...v1.4.0

## [1.3.0] - 2026-10-07

### What's Changed
* Tighten up .circleci/info.yml handling by @conormcd in https://github.com/CircleCI-Public/circleci-cli/pull/1876
* Report install method and build age in telemetry by @EnoshAnwar in https://github.com/CircleCI-Public/circleci-cli/pull/1880
* Report the latest release in `version --json` by @EnoshAnwar in https://github.com/CircleCI-Public/circleci-cli/pull/1885
* Drop the legacy prefix hint from function not-found errors by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1896
* Print the example step after function add by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1895
* Show a function's example config in function get by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1894


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.2.0...v1.3.0

## [1.2.0] - 2026-10-06

### What's Changed
* [PIPE-9912] Surface pipeline warnings across all CLI commands by @parkuman in https://github.com/CircleCI-Public/circleci-cli/pull/1869
* ONP-4197 | List `runner token` rows by resource class ID, without a lookup by @atulsingh0 in https://github.com/CircleCI-Public/circleci-cli/pull/1879


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.1.0...v1.2.0

## [1.1.0] - 2026-10-06

### What's Changed
* Release by merging a release PR, as the JetBrains plugin does by @pete-woods in https://github.com/CircleCI-Public/circleci-cli/pull/1886


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.0.52041...v1.1.0

## [1.0.52041] - 2026-10-05

The last release numbered by CI build. The notes for it and for earlier releases are on
[GitHub](https://github.com/CircleCI-Public/circleci-cli/releases).
