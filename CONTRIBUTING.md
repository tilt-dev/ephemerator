# Contributing

## Development

Create a KIND cluster that maps localhost:80 to the ingress node.

```
ctlptl apply -f ephconfig/cluster.yaml
```

Verify that all localhost subdomains resolve to the loopback address on your machine (127.0.0.1).

```
$ host subdomain.localhost
subdomain.localhost has address 127.0.0.1
subdomain.localhost has IPv6 address ::1
```

Bring up the controller in your KIND cluster with Tilt:

```
tilt up
```

Tilt will:

- Start the controller

- Deploy a configmap named `nicks`

- The controller will create a new pod that's owned by the configmap

- The ephemeral Tilt instance will be available at http://8000---nicks.preview.localhost/

- The ephemeral environment service will be available at http://10350---nicks.preview.localhost/

## Oauth

By default, the ephemerator will run locally without any authentication.

The username will always be 'nicks'.

Running with authentication requires some secrets. These secrets control github rate limiting.
If you work at Tilt Dev (the company), we have these secrets in the 1password.
But it's also easy to generate them for yourself!

To run with authentication, you'll need to:

- Create an oauth client ID and secret at https://github.com/settings/developers

- Create a cookie secret with the command: 

```
python -c 'import os,base64; print(base64.urlsafe_b64encode(os.urandom(32)).decode())'
```

- Create a file under `.secrets/ephoauth2.yaml`

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: ephoauth2
data:
  OAUTH2_PROXY_CLIENT_ID: YOUR_CLIENT_ID_FROM_GITHUB
  OAUTH2_PROXY_CLIENT_SECRET: YOUR_CLIENT_SECRET_FROM_GITHUB
  OAUTH2_PROXY_COOKIE_SECRET: YOUR_COOKIE_SECRET
  OAUTH2_PROXY_COOKIE_SECURE: "false"
```
