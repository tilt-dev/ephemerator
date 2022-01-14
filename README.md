# ephemerator

Ephemerate your life with [ephemeral
environments](https://ephemeralenvironments.io/).

The ephemerator shows how to create on-demand Kubernetes clusters
for demos and preview apps.

## Goals

We want this project to be just enough that a small team can use it to
operate ephemeral environments on their Kubernetes cluster.

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

## Architecture

The desired state of ephemeral environments in the cluster are stored in ConfigMaps
on the cluster itself with the label `app: ephemerator.tilt.dev`.

The ephemerator consists of three pieces:

`ephctrl` - A Kubernetes controller that continuously configmaps in the cluster
and creates the environments.

`ephdash` - A dashboard where users manage their environments.

`ephgateway` - The ingress that routes traffic to each environment.
  
The servers need the following permissions:

`ephctrl` - Read/write access on Deployments, Services, Ingresses, and ConfigMaps in its own namespace.

`ephdash` - Read/write access on ConfigMaps in its own namespace.

Both servers are written in Go, but could be written in any language with a
Kubernetes client library.

## License

TK Nick add a license
