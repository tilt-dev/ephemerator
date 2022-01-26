FROM alpine

ADD ./build/ephdash /usr/local/bin/ephdash

ENTRYPOINT /usr/local/bin/ephdash
