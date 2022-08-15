#!/usr/bin/env bash

set -eu

docker build --build-arg LINKERD_VERSION=2.11.4 -f ./injector/Dockerfile -t aatarasoff/easyauth-webhook:${TAG:-latest} .