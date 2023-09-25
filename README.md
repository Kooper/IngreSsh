# SSH ingress for Kubernetes

![build and test](https://github.com/Kooper/IngreSsh/actions/workflows/go.yml/badge.svg)

The project implements a Kubernetes ingress controller, which routes incoming
SSH connections to the shell sessions at authorized pods. Authorization and
routing are configured as IngreSsh Kubernetes resources.

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

[![asciicast](https://asciinema.org/a/gh6CTevs3p55ARhVcKLYNPizF.svg)](https://asciinema.org/a/gh6CTevs3p55ARhVcKLYNPizF)

## Configuration

### IngreSsh Resource

An elaborate description of the `IngreSsh` resources schema is available at [api/v1/ingressh_types.go](api/v1/ingressh_types.go)

Below is a brief outline of the resource:

```yaml
---
apiVersion: ingress.kuberstein.io/v1
kind: IngreSsh
metadata:
  name: ssh-exec
spec:
  session: Exec                # Uses exec command
  command: [/bin/sh]           # Uses /bin/sh as the user's shell
  selectors: [app=nginx]       # Authorizes access to the pods with this label in the namespace of the resource
  authorizedKeys:
  - ssh-rsa AAAAB3NzaC1yc2E... # Like ~/.ssh/authorized_keys
---
apiVersion: ingress.kuberstein.io/v1
kind: IngreSsh
metadata:
  name: ssh-debug
spec:
  session: Debug               # Uses debug attach command
  image: busybox               # Starts busybox ephemeral container to attach the user's shell
  selectors: [app=nginx]       # Authorizes access to the pods with this label in the namespace of the resource
  authorizedKeys:
  - ssh-rsa AAAAB3NzaC1yc2E... # Like ~/.ssh/authorized_keys
```

### Server Configuration

The server configuration consists of the server's RSA private key and configuration file.
When running from the source, they are defaulted to the sample configs in
[manifests/server](manifests/server)

To run the server from the docker container in the cluster you'll need a Secret and ConfigMap
resources for them:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: secret-ssh
type: kubernetes.io/ssh-auth
data:
  ssh-privatekey: |
    MIIEpQIBAAKCAQEAulqb/Y ...
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-ssh
data:
  ssh_server_config: |
    bind_address: ":2222"
    host_key_file: "ssh-privatekey"
    debug_image: "ubuntu"
---
# Here should go the pod definition for the server's container
```

## How to try it from the source

You’ll need a Kubernetes cluster to run against. You can use
[KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run
against a remote cluster.

**Note:** Your controller will automatically use the current context in your
kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

If you are going to use `kind` for experiments, then the following should be
enough:

```sh
kind create cluster
```

Install CRD:

```sh
kubectl apply -f manifests/k8s/crd.yaml
```

Run some pods:

```sh
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
ssh 127.0.0.1 -p 2222
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
