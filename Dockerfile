FROM cimg/go:1.17

ENV CIRCLECI_CLI_SKIP_UPDATE_CHECK true

COPY ./dist/circleci-cli_linux_amd64/circleci /usr/local/bin
