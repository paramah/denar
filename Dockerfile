# syntax=docker/dockerfile:1

# Web UI: React SPA built by Vite into web/dist, embedded by web/embed.go.
# It runs on the build platform only: the output is the same for every target.
# Keep the major version equal to web/.nvmrc (CI builds the UI with that one).
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json web/.npmrc ./
# postinstall writes node_modules/go.mod (keeps node_modules out of `go ./...`).
RUN npm ci --no-audit --no-fund
COPY web/ ./
# Some web sources refer to ../api (the OpenAPI contract).
COPY api/ /src/api/
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist/ ./web/dist/
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/denar ./cmd/denar

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
