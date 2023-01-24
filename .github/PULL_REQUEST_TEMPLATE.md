# Checklist

=========

- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have checked for similar issues and haven't found anything relevant.
- [ ] This is not a security issue (which should be reported here: https://circleci.com/security/)
- [ ] I have read [Contribution Guidelines](https://github.com/CircleCI-Public/circleci-cli/blob/main/CONTRIBUTING.md).

### Internal Checklist
- [ ] I am requesting a review from my own team as well as the owning team
- [ ] I have a plan in place for the monitoring of the changes that I am making (this can include new monitors, logs to be aware of, etc...)

## Changes

=======

- Put itemized changes here, preferably in imperative mood, i.e. "Add
  `functionA` to `fileB`"

## Rationale

=========

What was the overarching product goal of this PR as well as any pertinent
history of changes

## Considerations

==============

Why you made some of the technical decisions that you made, especially if the
reasoning is not immediately obvious

## Screenshots

============

<h4>Before</h4>
Image or [gif](https://giphy.com/apps/giphycapture)

<h4>After</h4>
Image or gif where change can be clearly seen

## **Here are some helpful tips you can follow when submitting a pull request:**

1. Fork [the repository](https://github.com/CircleCI-Public/circleci-cli) and create your branch from `main`.
2. Run `make build` in the repository root.
3. If you've fixed a bug or added code that should be tested, add tests!
4. Ensure the test suite passes (`make test`).
5. The `--debug` flag is often helpful for debugging HTTP client requests and responses.
6. Format your code with [gofmt](https://golang.org/cmd/gofmt/).
7. Make sure your code lints (`make lint`). Note: This requires Docker to run inside a local job.
