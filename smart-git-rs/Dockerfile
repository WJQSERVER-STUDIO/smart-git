FROM rust:1.94-bookworm AS build

WORKDIR /app

COPY Cargo.toml ./
COPY src ./src

RUN cargo build --release

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /data/smart-git

COPY --from=build /app/target/release/smart-git-rs /usr/local/bin/smart-git-rs

EXPOSE 8080

CMD ["smart-git-rs"]
