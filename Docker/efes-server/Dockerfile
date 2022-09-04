FROM efes-base
ADD entrypoint.sh /root/entrypoint.sh
ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
ADD config.toml /etc/efes.toml
EXPOSE 8500 8501
