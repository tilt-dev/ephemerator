FROM tiltdev/tilt

ADD tilt-upper-entrypoint.sh ./tilt-upper-entrypoint.sh

ENTRYPOINT ./tilt-upper-entrypoint.sh
