# Linkerd EasyAuth Extension

## Motivation
Simplify the Linkerd Authorization Policies management according to [the article](https://itnext.io/a-practical-guide-for-linkerd-authorization-policies-6cfdb50392e9) by giving a bunch of predefined policies and opinionated structures.

## Supported versions
- 2.11.x only

## How to use it

## CLI
Grab latest binaries from the releases page: https://github.com/aatarasoff/linkerd-easyauth/releases.

### Usage
```
linkerd easyauth [COMMAND] -n <namespace> [FLAGS]
```

### Supported commands
- `authcheck`: checks for obsolete `Server` and `ServerAuthorization` resources, checks that PODs ports have `Server` resource
- `list`: list of Pods that were injected by `linkerd.io/easyauth-enabled: true` annotation (more information below)

## Helm chart
Install the helm chart with injector and policies:
```
> kubectl create ns linkerd-easyauth

# Edit namespace and add standard linkerd annotations

> helm repo add linkerd-easyauth https://aatarasoff.github.io/linkerd-easyauth
> helm install -n linkerd-easyauth linkerd-easyauth linkerd-easyauth/linkerd-easyauth --values your_values.yml
```

### What the helm chart provides
- Injector that adds `linkerd.io/easyauth-enabled: true` label for all meshed pods (you can limit namespaces via helmchart)
- `Server` in terms of Linkerd authorization policies for `linkerd-admin-port`
- `ServerAuthorization` resources that provides basic allow policies for kubelet, ingress, Linkerd itself, and monitoring

### What the helm chart does not provide
Because the `Server` should be one per service per port, we can define the server for the linkerd proxy admin port only.
For each port that should be used by other pods, or Linkerd you should add the server definition manually:
```
---
apiVersion: policy.linkerd.io/v1beta1
kind: Server
metadata:
  namespace: <app-namespace>
  name: <app-server-name>
  labels:
    linkerd.io/server-type: common
spec:
  podSelector:
    matchLabels:
      <app-label>: <app-unique-value>
  port: <my-port-name>
``` 

### Important Values
#### Meshed Apps Namespaces
Because all `ServerAuthorization` policies are Namespaced scope then we should add common policies to each namespace with our apps:
```
meshedApps:
  namespaces:
    - hippos
    - elephants
```

#### Kubelet CIDR
Because of [the issue](https://github.com/linkerd/linkerd2/issues/7050), you should explicitly provide CIDR for kubelet.
It depends on the K8s implementation you are using.

There are two possibity. If you can define CIDR granually then you can use it
```
  kubelet:
    cidr:
      - cidr: 10.164.0.0/20
```

If you cannot do it but you have GKE-like pattern then you can define octets and ranges for generation the bunch of `/32` CIDR:
```
  kubelet:
    cidr: []
    # generate by pattern octet0:{low1-high1}:{low2-high2}:octet3 (10.169.150.1)
    generator:
      octet0: 10
      low1: 168
      high1: 172
      low2: 0
      high2: 256
      octet3: 1
```
