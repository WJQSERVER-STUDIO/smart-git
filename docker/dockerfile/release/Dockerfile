FROM alpine:latest AS builder

ARG USER=WJQSERVER-STUDIO
ARG REPO=smart-git
ARG APPLICATION=smart-git
ARG TARGETOS
ARG TARGETARCH
ARG TARGETPLATFORM

# 创建文件夹
RUN mkdir -p /data/${APPLICATION}/repos
RUN mkdir -p /data/${APPLICATION}/config 
RUN mkdir -p /data/${APPLICATION}/log 
RUN mkdir -p /data/${APPLICATION}/db

# 安装依赖
RUN apk add --no-cache curl wget tar

# 后端
RUN VERSION=$(curl -s https://raw.githubusercontent.com/${USER}/${REPO}/main/VERSION) && \
    wget -O /data/${APPLICATION}/${APPLICATION}-${TARGETOS}-${TARGETARCH}.tar.gz https://github.com/${USER}/${REPO}/releases/download/$VERSION/${APPLICATION}-${TARGETOS}-${TARGETARCH}.tar.gz && \
    tar -zxvf /data/${APPLICATION}/${APPLICATION}-${TARGETOS}-${TARGETARCH}.tar.gz -C /data/${APPLICATION} && \
    rm -rf /data/${APPLICATION}/${APPLICATION}-${TARGETOS}-${TARGETARCH}.tar.gz
RUN wget -O /usr/local/bin/init.sh https://raw.githubusercontent.com/${USER}/${REPO}/main/docker/dockerfile/release/init.sh

# 拉取配置
RUN wget -O /data/${APPLICATION}/config.toml https://raw.githubusercontent.com/${USER}/${REPO}/main/config/config.toml

# 权限
RUN chmod +x /data/${APPLICATION}/${APPLICATION}
RUN chmod +x /usr/local/bin/init.sh

FROM alpine:latest

RUN apk add --no-cache curl

COPY --from=builder /data/${APPLICATION} /data/${APPLICATION}
COPY --from=builder /usr/local/bin/init.sh /usr/local/bin/init.sh

# 权限
RUN chmod +x /data/${APPLICATION}/${APPLICATION}
RUN chmod +x /usr/local/bin/init.sh

CMD ["/usr/local/bin/init.sh"]

