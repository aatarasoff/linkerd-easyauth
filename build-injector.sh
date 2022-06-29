#!/usr/bin/env bash

set -eu

docker build --build-arg LINKER_VERSION=2.11.2 -f ./injector/Dockerfile -t aatarasoff/easyauth-webhook:${TAG:-latest} .