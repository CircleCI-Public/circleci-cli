FROM alpine:3.8

COPY ./dist/linux_amd64/circleci /usr/local/bin

ENTRYPOINT ["circleci"]
CMD ["--help"]
