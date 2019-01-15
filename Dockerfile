FROM circleci/golang:1.10.3

ENV CIRCLECI_CLI_SKIP_UPDATE_CHECK=1

COPY ./dist/linux_amd64/circleci /usr/local/bin