# SSH ingress for Kubernetes

![build and test](https://github.com/Kooper/IngreSsh/actions/workflows/go.yml/badge.svg)
[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#experimental)

The project implements a Kubernetes ingress controller, which routes incoming
SSH connections to the shell sessions at authorized pods. Authentication and
authorization are configured as IngreSsh Kubernetes resources.

## Description

_"How can I SSH into the running pod in Kubernetes?"_ is probably the first
question a new software developer asks a Kubernetes administrator. The usual
answer is _"You can't, but there is kubectl exec/kubectl cp which are doing
the same."_

Kubectl does the trick indeed, but it looks like people just have a warm fuzzy
feeling about connecting with the familiar SSH to any environment like
Linux. As there are no roadblocks to implementing this scenario with the
Kubernetes model and available SSH libraries, the project provides the
implementation of an SSH ingress controller for Kubernetes. The controller
can route incoming SSH connections to shell sessions started in the
context of the target pods.

* This might be useful for users not comfortable with kubectl or who have no
  kubectl configured/installed
* The user could access the container for the application's debug purposes
  without the API server being exposed outside the secured perimeter
* It is possible to configure a predefined debug image with all the required
  tools to be used for shell sessions. This allows the administrator to control
  what is running as debug containers without allowing users to run whatever
  they want or set up special security policies

Incoming SSH connections are authenticated with the authorized keys, configured
in the ingress resource parameters. Ingress resource also contains
authorization rules, limiting which pods or containers the user can
access.

A shell is opened either as an exec command in the target container or as an
attach session to the debug container started automatically upon incoming
connection in the Linux namespace of the target container.

The project is implemented with:

* [kubebuilder](https://book.kubebuilder.io/)
* [GliderLabs](https://github.com/gliderlabs/ssh) SSH libraries
* CharmBracelet [bubbletea](https://github.com/charmbracelet/bubbletea) libraries

## Demo

Connecting to the pod in the cluster using SSH ingress:

[![asciicast](https://asciinema.org/a/jefrygN6KZ5faiWoHjUfcEtkS.svg)](https://asciinema.org/a/jefrygN6KZ5faiWoHjUfcEtkS)

## Configuration

### IngreSsh Resource

An elaborate description of the `IngreSsh` resources schema is available at [api/v1/ingressh_types.go](api/v1/ingressh_types.go)

Below is a brief outline of the resource:

```yaml
---
# Access to any container of the pod with app=nginx label. The session starts with
# exec of /bin/sh binary from the container
apiVersion: ingress.kuberstein.io/v1
kind: IngreSsh
metadata:
  name: ssh-exec
spec:
  session: Exec                # Uses exec command
  command: [/bin/sh]           # Uses /bin/sh as the user's shell
  selectors: [app=nginx]       # Authorizes access to the pods with this label in the namespace of the resource
  authorizedKeys:
  - user: kooper                    # User login name for audit
    key: ssh-rsa AAAAB3NzaC1yc2E... # Like ~/.ssh/authorized_keys
---
# Access to any container of the pod with app=nginx label. The session starts with the
# ephemeral container running the "busybox" image in the Linux namespace of the target
# container. The default image entry point is used.
apiVersion: ingress.kuberstein.io/v1
kind: IngreSsh
metadata:
  name: ssh-debug
spec:
  session: Debug               # Uses debug attach command
  image: busybox               # Starts busybox ephemeral container to attach the user's shell
  selectors: [app=nginx]       # Authorizes access to the pods with this label in the namespace of the resource
  authorizedKeys:
  - user: kooper                    # User login name for audit
    key: ssh-rsa AAAAB3NzaC1yc2E... # Like ~/.ssh/authorized_keys
```

### Connecting

By default, if the user is authorized to access several targets, there is an
interactive selection of the target object.

[![asciicast](https://asciinema.org/a/e2gJS70bNEQrwMXEIA64SkpR1.svg)](https://asciinema.org/a/e2gJS70bNEQrwMXEIA64SkpR1)

```sh
# Connect using interactive namespace/pod/container selection for
# the authorized target container
$ ssh <cluster> -p <port>

# Connect to the specific namespace/pod/container. If all components
# are specified the interactive selection screen is skipped.
# However, you may specify only known components, narrowing the target
# selection like: namespace::@, :pod:@, ::container@, or any combination
# of those, like namespace:pod:@
# This would also skip interactive selection if the choice is unambiguous.
$ ssh <namespace>:<pod>:<container>@<cluster> -p <port>

# Connect to execute just a single command. Note that SSH does not set up
# a terminal for such connections, so no interactive selection of the target pod
# is available. You may specify the exact container with namespace:pod:container@
# part of the login string.
ssh <cluster> -p <port> /bin/cat /etc/nginx/nginx.config
```

### Server Configuration

The server configuration consists of the server's RSA private key and configuration file.
When running from the source, they are defaulted to the sample configs in
[manifests/server](manifests/server)

To run the server from the docker container in the cluster apply the ingress
controller configuration from [manifests/k8s](manifests/k8s).

**Note:** You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster
for testing. In this case, use the configuration with the port mapping:
`kind create cluster --config manifests/kind/config.yaml`

```sh
# Create global cluster resources
$ kubectl apply \
 -f manifests/k8s/clusterrole.yaml \
 -f manifests/k8s/namespace.yaml \
 -f manifests/k8s/crd.yaml

# In the configured namespace create and authorize service account
$ kubectl -n ingressh-controller apply \
 -f manifests/k8s/service_account.yaml \
 -f manifests/k8s/role_binding.yaml

# Generate and put your own private SSH server key into the namespaced secret
$ ssh-keygen -t ed25519 -f ssh-privatekey
$ kubectl -n ingressh-controller create secret generic ssh-secret --from-file=ssh-privatekey

# In the configured namespace add the SSH server config and start the server
$ kubectl -n ingressh-controller apply \
 -f manifests/k8s/configmap.yaml \
 -f manifests/k8s/deployment.yaml \
 -f manifests/k8s/service.yaml
```

## How to try it from the source

You’ll need a Kubernetes cluster to run against. You can use
[KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run
against a remote cluster.

**Note:** Your controller will automatically use the current context in your
kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

If you are going to use `kind` for experiments, then the following should be
enough when running from the source:

```sh
# Create a cluster with the default configuration
$ kind create cluster

# Install CRD:
$ kubectl apply -f manifests/k8s/crd.yaml

# Run some pods:
kubectl apply -f manifests/samples/nginx.yaml
```

Then put your authorized key as an element in the spec.authorizedKeys list
for sample IngreSsh resources in `manifests/samples/ingressh-exec.yaml` and
create the resource:

```sh
kubectl apply -f manifests/samples/ingressh-exec.yaml
```

Build and run the controller. This will run in the foreground, so switch to a
new terminal if you want to leave it running:

```sh
make run
```

In another console window run SSH to connect to the pod:

```sh
ssh 127.0.0.1 -p 30022
```

## Run with the docker image

After installing CRD, creating some pods, and modifying IngreSsh resource
putting your authorized key (see the previous section),
build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/ingressh:tag
```

Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/ingressh:tag
```

To UnDeploy the controller from the cluster:

```sh
make undeploy
```

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Status

The code is new and may change or be removed in future versions. Please try it out and provide feedback.
If it addresses a use case that is important to you please open an issue to discuss it further.
