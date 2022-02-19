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
  newTag: SOME_VERSION

resources:
- path/to/config/crd
- path/to/config/rbac
- path/to/config/manager
```

### Customizing Arguments

By default (if you're using the above kustomization), `cmmc` runs with the `--leader-select` flag.
You can check [`main.go`][main] for available args or run `go run ./ --help`.

```
Usage of cmmc:
  -health-probe-bind-address string
    	The address the probe endpoint binds to. (default ":8081")
  -help
    	Display usage
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -leader-elect
    	Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  -merge-source-max-concurrent-reconciles int
    	MergeSourceController - MaxConcurrentReconciles (default 1)
  -merge-target-max-concurrent-reconciles int
    	MergeTargetController - MaxConcurrentReconciles (default 1)
  -metrics-bind-address string
    	The address the metric endpoint binds to. (default ":8080")
  -zap-devel
    	Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
  -zap-encoder value
    	Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
    	Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
    	Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
```

You can override any arguments by providing patches to the configuration above:

```yaml
patches:
- patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/args
      value: [
        "--leader-select",
        "--merge-target-max-concurrent-reconciles", "1",
        "--merge-source-max-concurrent-reconciles", "2",
      ]
  target:
    name: controller-manager
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
[main]: https://github.com/cashapp/cmmc/blob/main/main.go#L53
