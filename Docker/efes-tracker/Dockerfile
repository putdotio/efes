FROM efes-base
RUN apt-get update && apt-get -y install fuse
ADD entrypoint.sh /root/entrypoint.sh
ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
EXPOSE 8001
