FROM tiltdev/tilt

RUN apt update && apt install -y python3 python2
ADD tilt-upper-entrypoint.sh ./tilt-upper-entrypoint.sh
ADD tilt-healthcheck.py ./tilt-healthcheck.py

ENTRYPOINT ./tilt-upper-entrypoint.sh
