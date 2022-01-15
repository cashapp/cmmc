# MergeSource

```yaml
apiVersion: config.cmmc.k8s.cash.app/v1beta1
kind: MergeSource
metadata:
  name: merge-map-roles-aws-auth
spec:
  selector:
    cmmc.k8s.cash.app/merge: "something"
  source:
    data: someKey
  target:
    name: our-merge-target
    data: someKey
```

- A `MergeSource` describes what `ConfigMap` resource we are watching with its `selector` field.
  So any `ConfigMap` with a label that matches `spec.selector` will be watched.
- The controller will read data from the `source.data` field on a matching `ConfigMap`
- The `MergeSource` will annotate the watched CMs so they know they are being watched.
- _This resource/controller does no mutatations of the data on any of the resources outside of
  the annotation!_
- Annotations are cleaned up when the resource is deleted.
- The MergeTarget at `spec.target.name` will watch for `MergeSource` resources with it as the target
  and read their aggregated states to attempt to write to the target ConfigMap.
