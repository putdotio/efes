FROM golang:1.23.1 AS builder

# Install FUSE in builder stage
RUN apt-get update && \
    apt-get -y install --no-install-recommends fuse && \
    rm -rf /var/lib/apt/lists/*

################################

FROM efes/base

# Copy FUSE binaries and libraries
COPY --from=builder /usr/bin/fusermount /usr/bin/
COPY --from=builder /usr/lib/*/libfuse.so.* /usr/lib/x86_64-linux-gnu/
COPY --from=builder /etc/fuse.conf /etc/

# Create necessary FUSE directories
RUN mkdir -p /tmp/fuse

COPY ./Docker/efes-tracker/entrypoint.sh /root/entrypoint.sh
COPY ./Docker/efes-tracker/config.toml /etc/efes.toml

ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
