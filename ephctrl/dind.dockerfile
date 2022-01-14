FROM docker:20.10.12-dind

ADD dind-entrypoint.sh ./dind-entrypoint.sh

ENTRYPOINT ./dind-entrypoint.sh
