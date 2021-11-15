# ConfigMap Merging Controller (cmmc)

`cmmc` is a k8s operator that operates in cluster and allows for the merging
of ConfigMap resources based on specific labelling rules and data validation.

## Why?

The impetus for building this is to have a GitOps friendly solution to manage
the [`cm kube-system/aws-auth`][1].

Other solutions involve either manual edits, which don't have cleanup, or fully
custom resources to manage Auth/Roles itself.

Our approach was to have a more generalized watching & merging of configMaps into
a specific target.

## Features

- Watch specific keys of ConfigMaps and merge their results
- JSON Schema Validation for the target ConfigMap
- Fully Configurable source/target selectors mix and match 
- Changes to resources are non-destructive and recoverable
- Permissions gated by namespace selectors (if desired)

## How

The operator is built on the [kubebuilder][2] library and has two controllers/reconcilers, 
one for the `MergeSource` resource and one for `MergeTarget`.

### Resources & Configuration

#### `MergeSource`

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

#### `MergeTarget`

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

## Demo (aws-auth)

Let's build a solution for managing `kube-system/aws-auth`.

#### 1. Create a `MergeTarget`

A `MergeTarget` describes the target `ConfigMap` that want to write the data to.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: config.cmmc.k8s.cash.app/v1beta1
kind: MergeTarget
metadata:
  name: kube-system-aws-auth
spec:
  target: kube-system/aws-auth
  data:
    mapRoles: {}
    mapUsers: {}
EOF
```

This says that we want to write/merge data to the `mapRoles` and `mapUsers` keys
of `kube-system/aws-auth`. Note, there is no auth, or initial value for these keys
in this example, but we can add this later on.

#### 2. Create a `MergeSource` for `mapRoles`

A `MergeSource` describes what `ConfigMap`s we are watching to write to the `target`. 
This one specifically looks for ConfigMap resources with the label:
`cmmc:k8s.cash.app/merge: "aws-auth-map-roles"`.

__`target.name`__ refers to the `MergeTarget` we created earlier.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: config.cmmc.k8s.cash.app/v1beta1
kind: MergeSource
metadata:
  name: aws-auth-map-roles
spec:
  selector:
    cmmc.k8s.cash.app/merge: "aws-auth-map-roles"
  source:
    data: mapRoles
  target:
    name: kube-system-aws-auth 
    data: mapRoles
EOF
```

#### 3. Create some sample ConfigMap sources

Let's create a sample configuration for two services/namespaces, `service-a` and `service-b`,
which need some role binding from AWS to K8S.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: service-a
---
apiVersion: v1
kind: Namespace
metadata:
  name: service-b
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-roles-mapping
  namespace: service-a
  labels:
    cmmc.k8s.cash.app/merge: "aws-auth-map-roles"
data:
  mapRoles: |
    - arn: arn:aws:iam::111122223333:role/external-user-service-a
      username: service-a-external
      groups:
      - service-a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-roles-mapping
  namespace: service-b
  labels:
    cmmc.k8s.cash.app/merge: "aws-auth-map-roles"
data:
  mapRoles: |
    - arn: arn:aws:iam::111122223333:role/external-user-service-b
      username: service-b-external
      groups:
      - service-b
EOF
```

#### 4. Check the resources 

##### Target

```sh
kubectl get cm -n kube-system aws-auth -o yaml
```

```yaml
apiVersion: v1
data:
  mapRoles: |
    - arn: arn:aws:iam::111122223333:role/external-user-service-b
      username: service-b-external
      groups:
      - service-b
    - arn: arn:aws:iam::111122223333:role/external-user-service-a
      username: service-a-external
      groups:
      - service-a
kind: ConfigMap
metadata:
  annotations:
    config.cmmc.k8s.cash.app/managed-by-merge-target: default/kube-system-aws-auth
  name: aws-auth
  namespace: kube-system
```

##### Statuses

```
# kubectl get mergetarget
NAME                   TARGET                 READY   STATUS                         VALIDATION
kube-system-aws-auth   kube-system/aws-auth   True    Target ConfigMap up to date.   1 MergeSources reporting valid data
```

```
# kubectl get mergesource
NAME                 READY   STATUS
aws-auth-map-roles   True    Data from 2 ConfigMap(s)
```

### Cleanup

```sh
kubectl delete ns service-a
kubectl delete ns service-b
kubectl delete mergesource aws-auth-map-roles
kubectl delete mergetarget kube-system-aws-auth
```

[1]: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
