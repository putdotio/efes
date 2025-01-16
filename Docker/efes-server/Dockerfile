FROM efes/base

COPY ./Docker/efes-server/entrypoint.sh /root/entrypoint.sh
COPY ./Docker/efes-server/config.toml /etc/efes.toml

RUN mkdir -p /srv/efes/dev1
VOLUME /srv/efes/dev1

ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
