FROM golang:1.26-alpine AS build

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /out/smart-git .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /data/smart-git

COPY --from=build /out/smart-git /usr/local/bin/smart-git
COPY config/config.toml /data/smart-git/config.toml
COPY docker/dockerfile/release/init.sh /usr/local/bin/init.sh

RUN mkdir -p /data/smart-git/repos /data/smart-git/config /data/smart-git/log /data/smart-git/db \
    && chmod +x /usr/local/bin/smart-git /usr/local/bin/init.sh \
    && rm -rf /var/cache/apk/*

EXPOSE 8080

CMD ["/usr/local/bin/init.sh"]
