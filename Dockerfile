FROM alpine:3.24.1

RUN apk -U upgrade --scripts=no apk-tools \
    && apk add --no-cache ca-certificates tzdata dumb-init \
    && rm -rf /var/cache/apk/*

ENV TZ=UTC

COPY gogo /usr/bin/gogo

ENTRYPOINT ["dumb-init", "--", "gogo"]
