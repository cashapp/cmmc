# ConfigMap Merging Controller (cmmc)

`CMMC` is a Kubernetes operator that allows for the combination
of ConfigMap resources.

Documentation at <https://cashapp.github.io/cmmc>.

## Why?

The impetus for building this is to have a GitOps friendly solution to manage
`kube-system/aws-auth`, a ConfigMap that binds AWS roles to K8S Roles in EKS ([link][1]).

Instead of solving the problem directly, the approach was to ask the question:

> If another tool existed that would make this problem trivial to solve that wasn't
> just for _this specific use-case_, what would it be?

[See the demo for managing `kube-system/aws-auth` here](/example/aws-auth).

## Features

- Watch specific keys of ConfigMaps and merge their results into a target.
- Changes to resources are non-destructive and recoverable
- Permissions gated by namespace selectors
- JSON Schema Validation for the target ConfigMap (possibly add other validation in the future).
- Fully Configurable source/target selectors mix and match
- Metrics exposed for how many resources are being watched/updated, and their states.

## Usage

- [See usage & deployment docs](/usage).
- Resource reference: [`MergeSource`](/resources/mergesource), [`MergeTarget`](/resources/mergetarget).

## Contributing

[See contributing docs](/contributing).

## License

Apache Licnese 2.0

[1]: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
