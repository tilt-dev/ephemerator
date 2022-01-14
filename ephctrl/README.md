# ephctrl

An ephemeral environment controller.

## Development

Create a KIND cluster that maps localhost:80 to the ingress node.

```
ctlptl apply -f cluster.yaml
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

- Deploy a configmap named `nicks-env`

- The controller will create a new pod that's owned by the configmap

- The ephemeral Tilt instance will be available at http://tilt.nicks-env.localhost/

- The ephemeral environment service will be available at http://8000.nicks-env.localhost/
