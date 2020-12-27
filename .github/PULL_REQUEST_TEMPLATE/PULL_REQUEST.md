- [ ] I have read [Contribution Guidelines](https://github.com/CircleCI-Public/circleci-cli/blob/master/CONTRIBUTING.md).
- [ ] I have checked for similar issues and haven't found anything relevant.
- [ ] This is not a security issue (which should be reported here: https://circleci.com/security/)

**Here are some helpful tips you can follow when submitting a pull request:**

1. Fork [the repository](https://github.com/CircleCI-Public/circleci-cli) and create your branch from `master`.
2. Run `make build` in the repository root.
3. If you've fixed a bug or added code that should be tested, add tests!
4. Ensure the test suite passes (`make test`).
5. The `--debug` flag is often helpful for debugging HTTP client requests and responses.
6. Format your code with [gofmt](https://golang.org/cmd/gofmt/).
7. Make sure your code lints (`make lint`). Note: This requires Docker to run inside a local job.
