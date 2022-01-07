FROM docker:20.10.12-dind

RUN apk update && apk add curl ca-certificates rsync socat

# Install kubectl client
RUN set -exu \
  && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
  && chmod +x ./kubectl \
  && mv ./kubectl /usr/local/bin/kubectl

# install kind
ENV KIND_VERSION=v0.11.1
RUN set -exu \
  && curl -fLo ./kind-linux-amd64 "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
  && chmod +x ./kind-linux-amd64 \
  && mv ./kind-linux-amd64 /usr/local/bin/kind

# install ctlptl
ENV CTLPTL_VERSION="0.7.0"
RUN curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v$CTLPTL_VERSION/ctlptl.$CTLPTL_VERSION.linux.x86_64.tar.gz | \
  tar -xzv -C /usr/local/bin ctlptl

# install tilt
ENV TILT_VERSION="0.23.4"
RUN curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$TILT_VERSION/tilt.$TILT_VERSION.linux.x86_64.tar.gz | \
  tar -xzv -C /usr/local/bin tilt
