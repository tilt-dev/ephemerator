#!/bin/bash

if [[ "$TILT_UPPER_PATH" == "" ]]; then
    echo "Missing TILT_UPPER_PATH"
    exit 1
fi

if [[ "$TILT_UPPER_REPO" == "" ]]; then
    echo "Missing TILT_UPPER_REPO"
    exit 1
fi

if [[ "$TILT_UPPER_BRANCH" == "" ]]; then
    echo "Missing TILT_UPPER_BRANCH"
    exit 1
fi

while ! docker ps
do
    echo waiting for Docker socket
    sleep 2
done

set -euxo pipefail

rm -fR ./src
mkdir -p src
git clone "$TILT_UPPER_REPO" src
cd src
git checkout "$TILT_UPPER_BRANCH"
cd "$TILT_UPPER_PATH"

export DO_NOT_TRACK="1"
ctlptl create registry ctlptl-registry --port=5000
ctlptl create cluster kind --registry=ctlptl-registry
tilt up --host=0.0.0.0
