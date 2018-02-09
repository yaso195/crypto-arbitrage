FROM alpine:latest

MAINTAINER Edward Muller <edward@heroku.com>

WORKDIR "/opt"

ADD .docker_build/crypto-try-arbitrage /opt/bin/crypto-try-arbitrage

CMD ["/opt/bin/crypto-try-arbitrage"]

