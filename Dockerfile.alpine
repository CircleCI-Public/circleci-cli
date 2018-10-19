FROM alpine:3.8

COPY ./dist/linux_amd64/circleci /usr/local/bin

RUN apk add --no-cache --upgrade git openssh ca-certificates

ENTRYPOINT ["circleci"]
