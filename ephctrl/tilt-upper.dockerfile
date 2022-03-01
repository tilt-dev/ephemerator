FROM tiltdev/tilt

RUN apt update && apt install -y python3 python2

RUN curl -sL https://deb.nodesource.com/setup_16.x | bash -
RUN apt install -y nodejs

RUN curl -sL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list && \
  apt-get update && apt-get install yarn

ADD tilt-upper-entrypoint.sh ./tilt-upper-entrypoint.sh
ADD tilt-healthcheck.py ./tilt-healthcheck.py

# Install k3d
RUN TAG=v5.2.2 curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | bash

ENTRYPOINT ./tilt-upper-entrypoint.sh
