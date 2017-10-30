# Base this docker container off the official golang docker image.
# Docker containers inherit everything from their base.
FROM golang

# Copy build
COPY efes /usr/local/bin/efesbin

# Copy default config file.
COPY config.toml /etc/efes.toml

EXPOSE 8001

ENTRYPOINT ["/usr/local/bin/efes", "tracker"]
