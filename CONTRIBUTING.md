# Contributing Guidelines

Contributions are always welcome; however, please read this document in its
entirety before submitting a Pull Request or Reporting a bug.

### Table of Contents

- [Reporting a bug](#reporting-a-bug)
  - [Security disclosure](#security-disclosure)
- [Creating an issue](#creating-an-issue)
- [Feature requests](#feature-requests)
- [Opening a pull request](#opening-a-pull-request)
- [Hall of Fame](#hall-of-fame)
- [Code of Conduct](#code-of-conduct)
- [License](#license)
- [Contributor license agreement](#contributor-license-agreement)

---------------

# Reporting a Bug

Think you've found a bug? Let us know!

### Security disclosure

Security is a top priority for us. If you have encountered a security issue
please responsibly disclose it by following our [security
disclosure](https://circleci.com/security/) document.

# Creating an Issue

Your issue must follow these guidelines for it to be considered:

#### Before submitting

- Check you’re on the latest version, we may have already fixed your bug!
- [Search our issue
  tracker](https://github.com/CircleCI-Public/circleci-cli/issues/search&type=issues)
  for your problem, someone may have already reported it

# Opening a Pull Request

To contribute, [fork](https://help.github.com/articles/fork-a-repo/)
`circleci`, commit your changes, and [open a pull
request](https://help.github.com/articles/using-pull-requests/).

Your request will be reviewed as soon as possible. You may be asked to make
changes to your submission during the review process.

#### Before submitting

- Test your change thoroughly
- If you changed a command's help text (`Short`, `Long`, `Example`, or its
  flags), the golden snapshots of `--help`/usage output under
  `internal/cmd/root/testdata/` will be out of date. Regenerate them with:

  ```sh
  go test ./internal/cmd/root/... -update
  ```

  Then review the diff with `git diff internal/cmd/root/testdata/` to confirm
  only the intended commands changed before committing the updated files.

# Hall of Fame

Have you reported a bug that was fixed or even sent a patch that fixed one?

First of all, you rock! Thank you so much for your help!

Please send us a pull request and add yourself to the [CONTRIBUTORS.md](./CONTRIBUTORS.md) hall of fame.


# Code of Conduct

All community members are expected to adhere to our [code of
conduct](./CODE_OF_CONDUCT.md).


# License

CircleCI's `circleci` is released under the [MIT License](./LICENSE).
