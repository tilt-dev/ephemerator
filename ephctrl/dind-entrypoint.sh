#!/bin/sh
set -eu

exec docker-init -- dockerd --host=unix:///var/run/docker.sock
