#!/bin/bash

cd "$(dirname "$(dirname "$0")")"

set -euxo pipefail

TODAY=$(date +"%Y-%m-%d")
SECONDS=$(date +"%s")
TAG="$TODAY-$SECONDS"
REPO="gcr.io/windmill-prod"

docker_build() {
    DOCKER_BUILDKIT=1 docker build --pull -f "$1" -t "$REPO/$2:$TAG" .
    docker push "$REPO/$2:$TAG"
}

kubectl apply -f ephconfig/ephconfig-prod.yaml --namespace=ephemerator

pushd oauth2-proxy
docker_build "Dockerfile" oauth2-proxy
helm upgrade --install ephoauth2-proxy ./chart --namespace=ephemerator \
     --values=../.secrets/values-prod.yaml \
     --set=image.repository="$REPO/oauth2-proxy" \
     --set=image.tag="$TAG"
popd

pushd ephctrl
make build-static
docker_build "Dockerfile" ephctrl
docker_build "dind.dockerfile" ephctrl-dind
docker_build "tilt-upper.dockerfile" ephctrl-tilt-upper
helm upgrade --install ephctrl ./chart --namespace=ephemerator \
     --values=../.secrets/values-prod.yaml \
     --set=image.repository="$REPO/ephctrl" \
     --set=image.tag="$TAG" \
     --set=dindImage.repository="$REPO/ephctrl-dind" \
     --set=dindImage.tag="$TAG" \
     --set=tiltUpperImage.repository="$REPO/ephctrl-tilt-upper" \
     --set=tiltUpperImage.tag="$TAG" \
     --set=auth.enabled=true \
     --set=gateway.scheme=https \
     --set=gateway.host=preview.tilt.build \
     --set=gateway.tlsSecretName=preview-tilt-build \
     --set=k3d.imageRegistry="gcr.io/windmill-public-containers/registry:2" \
     --set=k3d.imageK3s="gcr.io/windmill-public-containers/k3s:v1.22.6-k3s1" \
     --set=k3d.imageTools="gcr.io/windmill-public-containers/k3d-tools" \
     --set=k3d.imageLoadbalancer="gcr.io/windmill-public-containers/k3d-proxy"
popd

pushd ephdash
make build-static
docker_build "ephdash.dockerfile" ephdash
helm upgrade --install ephdash ./chart --namespace=ephemerator \
     --values=../.secrets/values-prod.yaml \
     --set=image.repository="$REPO/ephdash" \
     --set=image.tag="$TAG" \
     --set=auth.proxy=http://oauth2-proxy:4180
popd
