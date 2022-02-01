# ephemerator

Ephemerate your life with [ephemeral
environments](https://ephemeralenvironments.io/).

The ephemerator shows how to create on-demand preview environments
with ephemeral Kubernetes clusters.

## Goals

We want this project to be just enough that a small or medium-sized team can use
it to operate preview environments on their Kubernetes cluster.

We use it to demo a small set of whitelisted Tilt example projects.

## Non-Goals

There are many features that go into a "gold standard" env operator:

- Creating a new env [on every code
  change](https://ephemeralenvironments.io/features/dev-workflow/).

- Destroying / scaling down envs that aren't being used [to save on
  cost](https://ephemeralenvironments.io/features/cost-control/).

- Secrets needed to checkout private code and 
  [access control](https://ephemeralenvironments.io/features/security/)
  over the managed envs.
  
The ephemerator operator isn't trying to solve these problems right now.

## Development

See [CONTRIBUTING.md](CONTRIBUTING) for details on how to run
this locally or in your own cluster.

## Architecture

The desired state of ephemeral environments in the cluster are stored in ConfigMaps
on the cluster itself with the label `app: ephemerator.tilt.dev`.

The ephemerator consists of four servers:

`ephctrl` - A Kubernetes controller that continuously configmaps in the cluster
and creates the environments.

`ephdash` - A dashboard where users manage their environments.

`ephgateway` - The ingress that routes traffic to each environment.

`oauth2-proxy` - [An oauth2 proxy](https://oauth2-proxy.github.io/oauth2-proxy/)
for authenticating users. Can also be used for access control.
  
The servers need the following permissions:

`ephctrl` - Read/write access on Deployments, Services, Ingresses, and ConfigMaps in its own namespace.

`ephdash` - Read/write access on ConfigMaps in its own namespace.

The `ephctrl` and `ephdash` servers are written in Go. They could be written in
any language with a Kubernetes client library.

## License

TK Nick add a license
