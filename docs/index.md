# CMMC

ConfigMap Merging Controller (cmmc) is a Kubernetes operator that allows for the combination
of ConfigMap resources.

## Why?

The impetus for building this is to have a GitOps friendly solution to manage
`kube-system/aws-auth`, a ConfigMap that binds [AWS roles to K8S Roles in EKS][1].
Instead of solving the problem directly, the approach was to ask the question:
_If another tool existed that would make this problem trivial to solve that wasn't
just for this specific use-case what would it be?_

- [Example for managing `kube-system/aws-auth`](/example/aws-auth).
- [Usage & deployment docs](/usage).
- Configuration reference: [`MergeSource`](/resources/mergesource), [`MergeTarget`](/resources/mergetarget).

## Features

- Watch specific keys of ConfigMaps and merge their results into a target ConfigMap.
- Changes to existing resource state are non-destructive and recoverable.
- JSONSchema validation capability to ensure that an invalid ConfigMap cannot be written.
- Permissions optionally gated by namespace selectors.
- Fully Configurable source/target selectors mix and match.
- Metrics exposed for how many resources are being watched/updated, and their reconcile states.

## Contributing

[See contributing docs](/contributing).

## License

Apache Licnese 2.0

[1]: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
