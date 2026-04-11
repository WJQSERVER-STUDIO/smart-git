FROM rust:1-alpine AS build

WORKDIR /app

ARG TARGETARCH

RUN apk add --no-cache musl-dev build-base pkgconfig openssl-dev sqlite-dev

COPY smart-git-rs/Cargo.toml smart-git-rs/Cargo.lock ./
COPY smart-git-rs/src ./src

RUN --mount=type=cache,target=/usr/local/cargo/registry,id=cargo-registry \
    --mount=type=cache,target=/app/target,id=cargo-target-${TARGETARCH} \
    cargo build --release --locked && \
    cp /app/target/release/smart-git-rs /app/smart-git-rs

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /data/smart-git

COPY --from=build /app/smart-git-rs /usr/local/bin/smart-git-rs
COPY smart-git-rs/config.wanf /data/smart-git/config.wanf

EXPOSE 8080

CMD ["smart-git-rs", "-c", "/data/smart-git/config"]
