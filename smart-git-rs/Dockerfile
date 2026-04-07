FROM rust:1-alpine AS build

WORKDIR /app

RUN apk add --no-cache musl-dev build-base pkgconfig openssl-dev sqlite-dev

COPY smart-git-rs/Cargo.toml smart-git-rs/Cargo.lock ./
COPY smart-git-rs/src ./src

RUN rustup target add x86_64-unknown-linux-musl \
    && cargo build --release --target x86_64-unknown-linux-musl

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /data/smart-git

COPY --from=build /app/target/x86_64-unknown-linux-musl/release/smart-git-rs /usr/local/bin/smart-git-rs
COPY smart-git-rs/config.wanf /data/smart-git/config.wanf

EXPOSE 8080

CMD ["smart-git-rs", "-c", "/data/smart-git/config"]
