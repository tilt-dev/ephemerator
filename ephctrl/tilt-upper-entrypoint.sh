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
cd "$(dirname "$TILT_UPPER_PATH")"

export DO_NOT_TRACK="1"
export K3D_IMAGE_LOADBALANCER="gcr.io/windmill-public-containers/k3d-proxy"
export K3D_IMAGE_TOOLS="gcr.io/windmill-public-containers/k3d-tools"
k3d registry create --image=gcr.io/windmill-public-containers/registry:2
k3d cluster create --image=gcr.io/windmill-public-containers/k3s:v1.22.6-k3s1 --registry-use k3d-registry
tilt up -f "$(basename "$TILT_UPPER_PATH")" --host=0.0.0.0
