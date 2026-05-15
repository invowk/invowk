# syntax=docker/dockerfile:1
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.26-bookworm AS go-base

FROM node:24-bookworm-slim

COPY --from=go-base /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:${PATH}" \
    GOPATH="/go" \
    GOCACHE="/tmp/go-build-cache" \
    GOAMD64="v3" \
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

RUN mkdir -p "${GOCACHE}" "${GOPATH}/bin" \
    && chmod -R a+rwX "${GOPATH}"

COPY . .
ENV GOPROXY="off" \
    GOSUMDB="off" \
    GOFLAGS="-mod=vendor"

RUN GOPROXY="https://proxy.golang.org|direct" GOSUMDB="sum.golang.org" GOFLAGS="" go mod vendor \
    && go build -trimpath -o /tmp/invowk-warmup . \
    && go test ./internal/benchmark ./internal/app/modulesync ./internal/app/moduleops ./cmd/invowk -run=^$ -bench=^$ \
    && rm -rf /tmp/invowk-warmup "${GOCACHE}" \
    && chmod -R a+rwX "${GOPATH}" /workspace

CMD ["node", "scripts/bench-bmf.mjs", "--output", "/tmp/invowk.bmf.json", "--raw-go-output", "/tmp/go-bench.txt"]
