ARG BUILD_FROM=ghcr.io/home-assistant/base:latest
FROM $BUILD_FROM AS builder

RUN apk update && apk add --no-cache \
	g++ \
	gcc \
	libpcap-dev

COPY --from=golang:alpine /usr/local/go/ /usr/local/go/

WORKDIR /tmp/phev2mqtt
COPY phev2mqtt/ .
RUN /usr/local/go/bin/go build

FROM $BUILD_FROM
RUN apk update && apk add --no-cache \
	libpcap-dev

COPY --from=builder /tmp/phev2mqtt/phev2mqtt /opt/phev2mqtt

COPY run.sh /opt
RUN chmod a+x /opt/run.sh

CMD [ "/opt/run.sh" ]
