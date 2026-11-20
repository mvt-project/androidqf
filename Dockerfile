FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

ARG TARGETARCH=amd64
ARG VERSION=1.8.3

RUN apk add --no-cache ca-certificates gcompat libgcc wget \
    && case "${TARGETARCH}" in \
        amd64|arm64) ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && cd /usr/local/bin \
    && wget -O "androidqf_linux_${TARGETARCH}_${VERSION}" \
        "https://github.com/mvt-project/androidqf/releases/download/v${VERSION}/androidqf_linux_${TARGETARCH}_${VERSION}" \
    && wget -O /tmp/checksums.txt \
        "https://github.com/mvt-project/androidqf/releases/download/v${VERSION}/checksums.txt" \
    && grep "  androidqf_linux_${TARGETARCH}_${VERSION}$" /tmp/checksums.txt | sha256sum -c - \
    && mv "androidqf_linux_${TARGETARCH}_${VERSION}" androidqf \
    && rm /tmp/checksums.txt \
    && chmod +x /usr/local/bin/androidqf

WORKDIR /acquisition

ENTRYPOINT ["androidqf"]
CMD ["-output", "/output"]
