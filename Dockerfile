# Copyright (c) 2026 Circle Internet Services, Inc.
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
# SPDX-License-Identifier: MIT

FROM cimg/base:current

LABEL maintainer="Developer Experience Team <developer_experience@circleci.com>"

# TARGETPLATFORM is a buildx predefined build arg ("linux/amd64", "linux/arm64")
# and has to be re-declared to be usable in a COPY. GoReleaser's dockers_v2 pipe
# stages each platform's binary under that exact path in the build context, so
# this single COPY serves every platform in the manifest.
ARG TARGETPLATFORM

# The update notifier is TTY-gated already, but be explicit: an image is a
# pinned artifact, so "a newer release is available" is never actionable from
# inside a container.
ENV CIRCLE_NO_UPDATE_CHECK=1

# Nothing here RUNs, so the arm64 image is assembled without QEMU/binfmt: the
# binaries are cross-compiled by the build step above this pipe, and buildx only
# has to copy them. Adding a RUN would mean emulating the foreign architecture in
# CI — install packages by extending this image instead.
COPY $TARGETPLATFORM/circleci /usr/local/bin/circleci

# Deliberately no ENTRYPOINT: cimg/base's default shell is what makes the image
# usable as a `docker:` executor image, where an entrypoint of `circleci` would
# hijack every step the job runs. `docker run <image> circleci ...` works because
# the binary is on PATH.
