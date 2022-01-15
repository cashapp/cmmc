# Contributing

The operator is built on the [kubebuilder][1] library and has two controllers/reconcilers,
one for the `MergeSource` resource and one for `MergeTarget`.

```sh
git clone git@github.com:cashapp/cmmc.git
cd cmmc
```

## Running locally

You can run a local build of the operator and point it at whatever cluster you
want.

```sh
make install
make run
```

## Running tests locally

```sh
make test
```

[1]: https://book.kubebuilder.io/
