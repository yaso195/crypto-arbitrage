FROM alpine:latest

MAINTAINER Edward Muller <edward@heroku.com>

WORKDIR "/opt"

ADD .docker_build/crypto-arbitrage /opt/bin/crypto-arbitrage

CMD ["/opt/bin/crypto-arbitrage"]

