FROM cimg/go:1.23

LABEL maintainer="Developer Experience Team <developer_experience@circleci.com>"

ENV CIRCLECI_CLI_SKIP_UPDATE_CHECK=true

COPY ./dist/circleci-cli_linux_amd64_v1/circleci /usr/local/bin
