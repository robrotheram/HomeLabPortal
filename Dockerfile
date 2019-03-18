FROM alpine:3.6
RUN mkdir -p /HomeLabPortal
COPY config /HomeLabPortal/config
COPY HomeLabPortal /HomeLabPortal/HomeLabPortal
COPY static /HomeLabPortal/static
COPY templates /HomeLabPortal/templates
COPY traefik.toml /HomeLabPortal/traefik.toml

RUN apk --no-cache add ca-certificates
RUN set -ex; \
	apkArch="$(apk --print-arch)"; \
	case "$apkArch" in \
		armhf) arch='arm' ;; \
		aarch64) arch='arm64' ;; \
		x86_64) arch='amd64' ;; \
		*) echo >&2 "error: unsupported architecture: $apkArch"; exit 1 ;; \
	esac; \
	apk add --no-cache --virtual .fetch-deps libressl; \
	wget -O /usr/local/bin/traefik "https://github.com/containous/traefik/releases/download/v1.7.9/traefik_linux-$arch"; \
	apk del .fetch-deps; \
chmod +x /usr/local/bin/traefik

COPY entrypoint.sh /
RUN chmod +x /entrypoint.sh
CMD /entrypoint.sh 
