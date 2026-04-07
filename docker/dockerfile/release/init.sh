#!/bin/sh

set -eu

APPLICATION=smart-git

if [ ! -f /data/${APPLICATION}/config/config.toml ]; then
    cp /data/${APPLICATION}/config.toml /data/${APPLICATION}/config/config.toml
fi

exec /usr/local/bin/smart-git -cfg /data/${APPLICATION}/config/config.toml > /data/${APPLICATION}/log/run.log 2>&1
