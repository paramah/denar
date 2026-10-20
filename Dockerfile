# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
# Web UI (etap 007) dołączy tu etap budowania web/dist; do tego czasu
# binarium serwuje stronę zastępczą.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/denar ./cmd/denar

FROM alpine:3
RUN adduser -D -H -u 10001 denar && mkdir /data && chown denar /data
COPY --from=build /out/denar /usr/local/bin/denar
USER denar
ENV DENAR_ADDR=:8080 \
    DENAR_DB=/data/denar.db
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["denar"]
CMD ["serve"]
