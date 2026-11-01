#!/bin/sh
set -eu

data_directory=/var/lib/jkb

mkdir -p "$data_directory/files"
chown -R jkb:jkb "$data_directory"

exec su-exec jkb:jkb "$@"
