FROM tiltdev/tilt

RUN apt update && apt install -y git make curl libsnmp-dev g++ gcc libc6-dev

RUN curl -sSL https://dl.google.com/go/go1.17.6.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH=/root/go/bin:/usr/local/go/bin:$PATH

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
  sh -s -- -b $(go env GOPATH)/bin v1.43.0

# Install Helm
RUN curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
