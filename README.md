# ilert-kube-agent

[![Build Status](https://github.com/iLert/ilert-kube-agent/workflows/docker-release/badge.svg)](https://github.com/iLert/ilert-kube-agent/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/iLert/ilert-kube-agent)](https://goreportcard.com/report/github.com/iLert/ilert-kube-agent) [![GoDoc](https://godoc.org/github.com/iLert/ilert-kube-agent?status.svg)](https://godoc.org/github.com/iLert/ilert-kube-agent)
[![Docker Pulls](https://img.shields.io/docker/pulls/ilert/ilert-kube-agent.svg?maxAge=604800)](https://hub.docker.com/r/ilert/ilert-kube-agent)

ilert-kube-agent is a service that listens to the Kubernetes API
server and generates incidents about the health state of the pods and the nodes. (See examples in
the Alerts section below.)

### Usage

Simply build and run ilert-kube-agent inside a Kubernetes pod which has a
service account token that has read-only access to the Kubernetes cluster.

#### Kubernetes Deployment

To deploy this project, you can simply run `kubectl apply -f examples/standard` and a
Kubernetes service and deployment will be created.

**Note:** Google Kubernetes Engine (GKE) Users - GKE has strict role permissions that will prevent the kube-state-metrics roles and role bindings from being created. To work around this, you can give your GCP identity the cluster-admin role by running the following one-liner:

```sh
kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud info --format='value(config.account)')
```

### Helm Deployment

```sh
helm repo add ilert https://ilert.github.io/charts/
helm repo update

helm upgrade --install --namespace kube-systems \
    ilert-kube-agent ilert/ilert-kube-agent \
    --set config.settings.apiKey="<YOUR KEY HERE>"
```

### Terraform Deployment

- Define module:

```hcl
module "ilert-kube-agent" {
  source  = "iLert/ilert-kube-agent/kubernetes"
  version = "0.3.5"
  replicas = 2
  api_key = "<YOUR KEY HERE>"
}
```

- Apply changes

```sh
terraform init
terraform apply
```

## Getting help

We are happy to respond to [GitHub issues][issues] as well.

[issues]: https://github.com/iLert/ilert-kube-agent/issues/new

<br>

#### License

<sup>
Licensed under <a href="LICENSE">Apache License, Version
2.0</a>
</sup>

<br>

<sub>
Unless you explicitly state otherwise, any contribution intentionally submitted for inclusion in ilert-kube-agent by you, as defined in the Apache-2.0 license, shall be dual licensed as above, without any additional terms or conditions.
</sub>
