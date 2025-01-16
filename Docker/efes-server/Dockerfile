FROM efes/base

COPY ./Docker/efes-server/entrypoint.sh /root/entrypoint.sh
COPY ./Docker/efes-server/config.toml /etc/efes.toml

ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
