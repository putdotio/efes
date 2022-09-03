FROM ubuntu:xenial

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get -y install rabbitmq-server

ARG RABBITMQ_USER=efes
ARG RABBITMQ_PASS=123

ENV RABBITMQ_NODENAME=efes@localhost

RUN rabbitmq-plugins enable --offline rabbitmq_management
EXPOSE 15671 15672

ADD init.sh /tmp/
RUN ["bash", "/tmp/init.sh"]

ENTRYPOINT ["rabbitmq-server", "--hostname", "localhost"]
