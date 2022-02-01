# Usage

!!! info cmmc version

    Be sure to use the correct version of the operator and the resources!

## Installation

A very basic install of the operator involves installing the [CRDs], the permissions for
the CMMC ServiceAccount to be able to watch and modify ConfigMaps, and the controller
manager itself.

* All of these [Kustomization][1] resources are located in the [`config/`][2] directory.
* Each [release](https://github.com/cashapp/cmmc/releases) of cmmc is also pushed
  to [dockerhub](https://hub.docker.com/r/cashapp/cmmc/tags).


```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: cmmc-system
namePrefix: cmmc-

images:
- name: controller
  newName: cashapp/cmmc
  newTag: v0.0.2

resources:
- path/to/config/crd
- path/to/config/rbac
- path/to/config/manager
```

## Metrics

| Metric | Type | Description |
| ------ | ---- | ----------- |
| `cmmc_resource_condition` | `gauge` | The current condition of the CMMC Resource. |
| `cmmc_resource_sources` | `gauge` | Number of sources per resource. |


You can add a [Prometheus](https://prometheus.io/) Monitor to scrape the metrics by
adding [`config/prometheus`][3] to the list.

## Custom Resources

| Resource | Purpose | |
| -------- | ------- | |
| `MergeTarget` | Manages and validates a target ConfigMap | [docs](/cmmc/resources/mergetarget) |
| `MergeSource` | Watches source ConfigMaps for changes and accumulates their data | [docs](/cmmc/resources/mergesource) |

[1]: https://kubectl.docs.kubernetes.io/guides/introduction/
[2]: https://github.com/cashapp/cmmc/tree/main/config
[3]: https://github.com/cashapp/cmmc/blob/main/config/prometheus/monitor.yaml
[crds]: https://github.com/cashapp/cmmc/tree/main/config/crds
[metrics-issue]: https://github.com/cashapp/cmmc/issues/1
