#
# Builder Stage
#

FROM golang:1.26-trixie AS builder

WORKDIR /build

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
    && rm -rf /var/lib/apt/lists/*

COPY app/go.mod app/go.sum ./
RUN go mod download

COPY app/ ./

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w \
        -X main.version=${VERSION} \
        -X main.gitCommit=${GIT_COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -trimpath \
    -o replikator \
    ./cmd/replikator

#
# Final Image - Distroless Static (Enterprise-Grade Security)
#

FROM gcr.io/distroless/static-debian13:nonroot AS final

LABEL org.opencontainers.image.title="Replikator" \
      org.opencontainers.image.description="Kubernetes Operator for replicating secrets and configmaps across namespaces/clusters" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/cloudresty/replikator" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.vendor="CloudResty" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian13:nonroot" \
      securitylabel="enterprise-grade"

COPY --from=builder /build/replikator /

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nonroot

WORKDIR /

EXPOSE 8080 8081

ENTRYPOINT ["/replikator"]
