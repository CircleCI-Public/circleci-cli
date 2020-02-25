FROM circleci/golang:1.10.3

COPY ./dist/circleci-cli_linux_amd64/circleci /usr/local/bin
