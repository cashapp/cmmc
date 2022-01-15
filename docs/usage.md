# Usage

## Installation

* All of these [Kustomization][1] resources are located in the [`config/`][2] directory.
* Each [release](https://github.com/cashapp/cmmc/releases) of cmmc is also pushed to [dockerhub](https://hub.docker.com/r/cashapp/cmmc/tags).

!!! info cmmc version

    Be sure to use the correct version of the operator and the resources!

A very basic install of the operator involves installing the [CRDs], the permissions for the CMMC
ServiceAccount to be able to watch and modify ConfigMaps, and the controller manager itself.

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

You can also add a `PrometheusMonitor` to scrape the metrics by adding [`config/prometheus`][3]
to the list.

[FIXME][metrics-issue]

## Custom Resources

### MergeSource & MergeTarget

See [`MergeTarget`](/resources/mergetarget) and [`MergeSource`](/resources/mergesource) docs.

[1]: https://kubectl.docs.kubernetes.io/guides/introduction/
[2]: https://github.com/cashapp/cmmc/tree/main/config
[3]: https://github.com/cashapp/cmmc/blob/main/config/prometheus/monitor.yaml
[crds]: https://github.com/cashapp/cmmc/tree/main/config/crds
[metrics-issue]: https://github.com/cashapp/cmmc/issues/1
