# Apapo API. Built from the repository root (Nobus Cube looks for this file here).
# The Go code lives in backend/.

FROM golang:1.24-alpine AS build
WORKDIR /src
ARG VERSION=dev
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.buildVersion=${VERSION}" -o /out/apapo-api ./cmd/api

FROM alpine:3.20
# No package installs: CA certificates come from the build stage, so the runtime build
# needs no network. Times are UTC, so no tzdata is needed.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN addgroup -S -g 10001 apapo && adduser -S -D -H -u 10001 -G apapo apapo
WORKDIR /app
COPY --from=build /out/apapo-api /app/apapo-api
USER apapo:apapo
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=4s --start-period=5s --retries=3 CMD ["/app/apapo-api", "-healthcheck"]
CMD ["/app/apapo-api"]
