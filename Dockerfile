FROM alpine:3.19

# Install common utilities
RUN apk add --no-cache \
    bash \
    coreutils \
    go \
    git \
    make

WORKDIR /workspace

# Default command
CMD ["/bin/sh"]

