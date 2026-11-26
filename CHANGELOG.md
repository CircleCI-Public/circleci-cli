# Changelog

All notable changes to the CircleCI CLI will be documented in this file. Each release's section
is added by its release PR (see [RELEASE.md](RELEASE.md)) and becomes its GitHub release notes.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.20.0] - 2026-11-26

### What's Changed
* Accept telemetry attributes from extensions by @joaquin-sm-cci in https://github.com/CircleCI-Public/circleci-cli/pull/2032


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.19.0...v1.20.0

## [1.19.0] - 2026-11-25

### What's Changed
* Expose extensions to generated MCP by @liamclarkedev in https://github.com/CircleCI-Public/circleci-cli/pull/2016
* Show the rerun-failed command in run get by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/2018
* Document the configoptimizer extension in the command reference by @cloverfern in https://github.com/CircleCI-Public/circleci-cli/pull/2025
* fix: filter on_hold runs by the held phase and drop the pre-held approval fallbacks by @shawon-crosen in https://github.com/CircleCI-Public/circleci-cli/pull/2011
* Show job errors in job get and run get --failure-report by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/2028
* SC-113: Add --rules to project trigger create and update by @Crevitus in https://github.com/CircleCI-Public/circleci-cli/pull/2019
* Connect a Forgejo repo in circleci onboard (AEX-97) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1997
* Accept CircleCI URLs in run get and the job commands (AEX-80) by @alexandrahsu-eng in https://github.com/CircleCI-Public/circleci-cli/pull/1965
* Show the backend reason when a workflow rerun is refused by @alexandrahsu-eng in https://github.com/CircleCI-Public/circleci-cli/pull/1957
* Add per-job timing to run get and workflow get, and run get --timing (AEX-101) by @alexandrahsu-eng in https://github.com/CircleCI-Public/circleci-cli/pull/2030

### New Contributors
* @Crevitus made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/2019
* @alexandrahsu-eng made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/1965

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.18.0...v1.19.0

## [1.18.0] - 2026-11-19

### What's Changed
* Move run trigger and pipeline run onto the v3 runs API by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/2006
* Add test step using testsuite to generated config by @morganmahan in https://github.com/CircleCI-Public/circleci-cli/pull/2008
* Say when a failed request may still have taken effect by @EnoshAnwar in https://github.com/CircleCI-Public/circleci-cli/pull/2013
* CF2-993: Let extensions shut down cleanly on Ctrl+C / SIGTERM by @joaquin-sm-cci in https://github.com/CircleCI-Public/circleci-cli/pull/1983

### New Contributors
* @morganmahan made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/2008

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.17.0...v1.18.0

## [1.17.0] - 2026-11-18

### What's Changed
* Stop replaying POST and PATCH requests the server may have acted on by @EnoshAnwar in https://github.com/CircleCI-Public/circleci-cli/pull/2010


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.16.0...v1.17.0

## [1.16.0] - 2026-11-18

### What's Changed
* Record extension subcommand and exit code on command_invocation by @Fab10-CircleCi in https://github.com/CircleCI-Public/circleci-cli/pull/1964
* Check pipeline create's GitHub Actions config path (AEX-133) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1979
* Move CLI provider lists into the provider registry (AEX-97) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1985
* Accept Forgejo in pipeline create and trigger create (AEX-97) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1986
* List an empty job's artifacts as empty instead of not found (AEX-99) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1987
* Add run artifacts and run list --status (AEX-99) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1988
* Let function commands run without login by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1996
* Show hostname, IP and last task time on runner instance list by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1995
* Add --ssh and --job to workflow rerun by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1998
* Add org get with admin status by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1999
* Print a prompt for your coding agent from config generate by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1994
* Add runner terms accept by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/2000
* fix: treat held approval jobs as pending and render the held phase by @shawon-crosen in https://github.com/CircleCI-Public/circleci-cli/pull/2003
* Print the ssh command after workflow rerun --ssh by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/2001
* Repoint glamour replace at the rebased fork commit by @pete-woods in https://github.com/CircleCI-Public/circleci-cli/pull/2007
* Bump github.com/go-chi/chi/v5 from 5.3.2 to 5.4.0 by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1956
* Record when config validate shows the Functions tip by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/2004


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.15.0...v1.16.0

## [1.15.0] - 2026-11-10

### What's Changed
* Add testresult flaky to list a project's flaky tests by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1969
* Say --type is only for Functions, not MCP tools or CLI commands by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1974
* Add extension reference to prompt commands by @liamclarkedev in https://github.com/CircleCI-Public/circleci-cli/pull/1977
* Send project slugs with literal slash separators by @shawon-crosen in https://github.com/CircleCI-Public/circleci-cli/pull/1978
* Add --failure-report to my runs by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1976
* Emit RFC 3339 timestamps in JSON and honour TZ in human output by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1981
* Suggest Functions when config validate passes on a new config by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1975

### New Contributors
* @shawon-crosen made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/1978

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.14.0...v1.15.0

## [1.14.0] - 2026-11-04

### What's Changed
* TEST-376 Add device and user ID to extension env by @liamclarkedev in https://github.com/CircleCI-Public/circleci-cli/pull/1959
* Stamp source builds with git describe so they aren't nagged to upgrade by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1958
* Escape markdown in function get and list output by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1961
* Point AI coding agents to CircleCI's skills (AEX-82) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1968


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.13.0...v1.14.0

## [1.13.0] - 2026-10-30

### What's Changed
* Warn that runner token create no longer prints runner config YAML (ONP-4112) by @olcarl-circleci in https://github.com/CircleCI-Public/circleci-cli/pull/1950
* Only cascade resource-class delete to tokens with --force by @soulchips in https://github.com/CircleCI-Public/circleci-cli/pull/1927
* Unhide circleci function by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1949


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.12.0...v1.13.0

## [1.12.0] - 2026-10-28

### What's Changed
* Point run watch failures at the failure report and next steps (AEX-42) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1937
* Add circleci preflight to run uncommitted changes through CI by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1889
* End the failure report with next steps by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1941
* Add project trigger update and delete by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1951
* Show job warnings in job get by @felixshiftellecon in https://github.com/CircleCI-Public/circleci-cli/pull/1953
* Let feedback report a missing or short Function by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1954
* Select pipeline definition by name in pipeline run by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1948
* Add circleci orb version to get a single orb version by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1946

### New Contributors
* @felixshiftellecon made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/1953

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.11.0...v1.12.0

## [1.11.0] - 2026-10-21

### What's Changed
* Add circleci notification slack commands by @DominikLa in https://github.com/CircleCI-Public/circleci-cli/pull/1916
* Bump github.com/getkin/kin-openapi from 0.144.0 to 0.149.0 in /tools by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1928

### New Contributors
* @DominikLa made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/1916

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.10.0...v1.11.0

## [1.10.0] - 2026-10-21

### What's Changed
* Add circleci feedback for reporting CLI problems (AEX-48) by @joel-thompson in https://github.com/CircleCI-Public/circleci-cli/pull/1911

### New Contributors
* @joel-thompson made their first contribution in https://github.com/CircleCI-Public/circleci-cli/pull/1911

**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.9.0...v1.10.0

## [1.9.0] - 2026-10-20

### What's Changed
* Bump github.com/klauspost/compress from 1.18.6 to 1.18.7 in /tools by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1933
* Bump github.com/shirou/gopsutil/v4 in / by @dependabot[bot] in https://github.com/CircleCI-Public/circleci-cli/pull/1915
* Bump golangci-lint to v2.15.0 by @pete-woods in https://github.com/CircleCI-Public/circleci-cli/pull/1935


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.8.0...v1.9.0

## [1.8.0] - 2026-10-20

### What's Changed
* Show the arguments of a function's default command by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1924
* Add run approve to approve an approval job on hold by @gcirclest in https://github.com/CircleCI-Public/circleci-cli/pull/1930
* Bump Go to 1.27.3 by @meeech in https://github.com/CircleCI-Public/circleci-cli/pull/1932


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.7.0...v1.8.0

## [1.7.0] - 2026-10-16

### What's Changed
* Bind positional resource class on runner token list by @soulchips in https://github.com/CircleCI-Public/circleci-cli/pull/1918
* Bind positional namespace on runner resource-class list by @soulchips in https://github.com/CircleCI-Public/circleci-cli/pull/1919
* Bind positional namespace or resource class on runner instance list by @soulchips in https://github.com/CircleCI-Public/circleci-cli/pull/1920
* Treat commit-suffixed function versions as builds by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1923


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.6.0...v1.7.0

## [1.6.0] - 2026-10-15

### What's Changed
* Update CircleCI config to use new setup-go version by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1875
* List function pseudo-versions only on request by @stiyyagura0901 in https://github.com/CircleCI-Public/circleci-cli/pull/1906
* [ONP-4286] Show task counts on runner fleet list by @christian-stephen in https://github.com/CircleCI-Public/circleci-cli/pull/1917


**Full Changelog**: https://github.com/CircleCI-Public/circleci-cli/compare/v1.5.0...v1.6.0

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
