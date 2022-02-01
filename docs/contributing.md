# Contributing

The operator is built on the [kubebuilder][1] library and has two controllers/reconcilers,
one for the `MergeSource` resource and one for `MergeTarget`, if you haven't gone through
the book, there are some very useful examples for how to get an operator running locally.

## Requirements

- **Hermit.** The library uses [hermit][3] for local dependency management,
   see [installation instructions][4].
- **Docker / Kubernetes.**  You'll generally need `docker` installed and have
   a k8s cluster available for testing and iteration.

## Running Locally

Once you have the repo which you can either clone directly, or fork.

```sh
cd path/to/cmmc
```

If hermit is installed with the environment detection you should immediately see
a _hermit environment prompt_.

```sh
# generate the CRDs and install them into the current kubernetes cluster
make install

# run the operator locally against the kubernetes cluster
make run

# to run automated tests
make test
```

## Documentation

To run a development version of the documentation server.

```sh
make mkdocs-serve
```

## The `Makefile`

```sh
make help

Usage:
  make <target>

General
  help             Display this help.

Development
  manifests        Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
  generate         Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
  fmt              Run go fmt against code.
  vet              Run go vet against code.
  lint             Run golangci-lint against the code.
  mkdocs-serve     Run mkdocs-material dev server in docker.
  test             Run tests.

Build
  build            Build manager binary.
  run              Run a controller from your host.
  docker-build     Build docker image with the manager.
  docker-push      Push docker image with the manager.

CI
  diff-check       Checks to see if there are any changes in git

Deployment
  install          Install CRDs into the K8s cluster specified in ~/.kube/config.
  uninstall        Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
  deploy           Deploy controller to the K8s cluster specified in ~/.kube/config.
  undeploy         Undeploy controller from the K8s cluster specified in ~/.kube/config.
  controller-gen   Download controller-gen locally if necessary.
  kustomize        Download kustomize locally if necessary.
```

[1]: https://book.kubebuilder.io/
[2]: https://github.com/cashapp/cmmc
[3]: https://github.com/cashapp/hermit
[4]: https://cashapp.github.io/hermit/usage/get-started/
