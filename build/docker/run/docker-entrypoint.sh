#!/bin/dash
set -e

envsubst </opt/blockchaincfg.json.template >/tmp/blockchaincfg.json

exec "$@"
