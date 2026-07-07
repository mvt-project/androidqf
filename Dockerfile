FROM alpine:3.23

ARG TARGETARCH=amd64
ARG VERSION=1.8.3

RUN apk add --no-cache ca-certificates gcompat libgcc wget \
    && case "${TARGETARCH}" in \
        amd64|arm64) ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && wget -O /usr/local/bin/androidqf \
        "https://github.com/mvt-project/androidqf/releases/download/v${VERSION}/androidqf_linux_${TARGETARCH}_${VERSION}" \
    && chmod +x /usr/local/bin/androidqf

WORKDIR /acquisition

ENTRYPOINT ["androidqf"]
CMD ["-output", "/output"]
