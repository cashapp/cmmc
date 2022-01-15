# aws-auth

Let's build a solution for managing [`kube-system/aws-auth`][1].

## 1. Create a `MergeTarget`

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
of `kube-system/aws-auth`. Note, there is no validation, or initial value for these keys
in this example, but we can add this later on.

## 2. Create a `MergeSource` for `mapRoles`

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

## 3. Create some sample ConfigMap sources

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

## 4. Check the resources

### Target

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

### Statuses

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

## Cleanup

```sh
kubectl delete ns service-a
kubectl delete ns service-b
kubectl delete mergesource aws-auth-map-roles
kubectl delete mergetarget kube-system-aws-auth
```

[1]: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
