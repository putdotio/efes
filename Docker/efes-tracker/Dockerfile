FROM efes-base
RUN apt-get update && apt-get -y install fuse

COPY ./Docker/efes-tracker/entrypoint.sh /root/entrypoint.sh
COPY ./Docker/efes-tracker/config.toml /etc/efes.toml

ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
EXPOSE 8001
