FROM tiltdev/tilt

RUN apt update && apt install -y python3 python2
ADD tilt-upper-entrypoint.sh ./tilt-upper-entrypoint.sh
ADD tilt-healthcheck.py ./tilt-healthcheck.py

# Install k3d
RUN TAG=v5.2.2 curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | bash

ENTRYPOINT ./tilt-upper-entrypoint.sh
