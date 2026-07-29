FROM alpine:3.23.5@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG TARGETARCH=amd64
ARG VERSION=1.8.3

RUN apk add --no-cache ca-certificates gcompat libgcc wget \
    && case "${TARGETARCH}" in \
        amd64|arm64) ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && wget -O /usr/local/bin/androidqf \
        "https://github.com/mvt-project/androidqf/releases/download/v${VERSION}/androidqf_linux_${TARGETARCH}_${VERSION}" \
    && wget -O /tmp/checksums.txt \
        "https://github.com/mvt-project/androidqf/releases/download/v${VERSION}/checksums.txt" \
    && cd /usr/local/bin \
    && grep "  androidqf_linux_${TARGETARCH}_${VERSION}$" /tmp/checksums.txt | sha256sum -c - \
    && rm /tmp/checksums.txt \
    && chmod +x /usr/local/bin/androidqf

WORKDIR /acquisition

ENTRYPOINT ["androidqf"]
CMD ["-output", "/output"]
