FROM alpine

ADD ./build/ephctrl /usr/local/bin/ephctrl

ENTRYPOINT /usr/local/bin/ephctrl
