FROM debian:stable-slim

# Install common utilities
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    coreutils \
    golang-go \
    git \
    make \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

# Default command
CMD ["/bin/bash"]
