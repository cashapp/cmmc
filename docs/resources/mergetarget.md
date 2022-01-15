# MergeTarget

```yaml
apiVersion: config.cmmc.k8s.cash.app/v1beta1
kind: MergeTarget
metadata:
  name: our-merge-target
spec:
  target: some-ns/some-resource-name # a configMap
  data:
    someKey:
      init: ''
      jsonSchema: |
        { … }
```

- A `MergeTarget` describes the resource we are managing, in this case it is `some-ns/some-resource-name`.
- We can configure this resource with the keys that we care about managing on the target, above
  its `someKey`.
- Each `data[$key]`
  - Can have an initial value that we'll inject _if the data was not present_ the key was missing or empty
  - Can have an optional `jsonSchema` that we use to validate the data _before it is persisted_.
- Creates the ConfigMap if it doesn't exist.
- Uses annotations to make sure there is only one `MergeTarget` per `spec.target`
- Clean up after itself when it is deleted.
  - If it didn't eist, it will be removed
  - If it did exist, the data will be reset back to what it was before.

