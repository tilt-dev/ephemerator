#!/bin/python
#
# Checks if tilt is running on 10350

import subprocess

try:
  subprocess.check_output(['tilt', 'get', 'uisession'])
except subprocess.CalledProcessError:
  exit(1)
