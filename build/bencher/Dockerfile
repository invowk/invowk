# syntax=docker/dockerfile:1
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.26-bookworm AS go-base

FROM node:24-bookworm-slim

COPY --from=go-base /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:${PATH}" \
    GOTOOLCHAIN=local \
    GOPROXY="https://proxy.golang.org|direct"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        gcc \
        git \
        libc6-dev \
        make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["node", "scripts/bench-bmf.mjs", "--output", "/tmp/invowk.bmf.json", "--raw-go-output", "/tmp/go-bench.txt"]
