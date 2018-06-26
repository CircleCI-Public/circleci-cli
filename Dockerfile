FROM circleci/golang:1.10.3

COPY ./dist/linux_amd64/circleci /usr/local/bin
