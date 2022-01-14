# Checks if all the currently running tilt resources are green.
#
# Usage:
# python3 tilt-healthcheck.py

import json
import os
import subprocess
import time
import datetime as datetime
import sys

ui_resource_list_json = '{}'
try:
  ui_resource_list_json = subprocess.check_output(['tilt', 'get', 'uiresources', '-o=json'])
except e:
  print('waiting for tilt: %s' % e)
  exit(1)

ui_resource_list = sorted(
  json.loads(ui_resource_list_json).get('items', []),
  key=lambda item: item['metadata']['name'])

pad = 20
def row(a, b, c, d):
  print('%s%s%s%s' % (a.ljust(pad), b.ljust(pad), c.ljust(pad), d.ljust(pad)))

row('Name', 'Update', 'Runtime', 'Overall')
has_failure = False
for r in ui_resource_list:
  name = r['metadata']['name']
  update_status = r.get('status', {}).get('updateStatus', '')
  runtime_status = r.get('status', {}).get('runtimeStatus', '')

  overall = 'PASS'
  if update_status != 'ok' and update_status != 'not_applicable':
    overall = 'FAIL'
    has_failure = True

  if runtime_status != 'ok' and runtime_status != 'not_applicable':
    overall = 'FAIL'
    has_failure = True

  row(name, update_status, runtime_status, overall)

if has_failure:
  exit(1)
