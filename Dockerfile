FROM circleci/golang:1.10.3

COPY build/linux/amd64/circleci-cli /usr/local/bin/
