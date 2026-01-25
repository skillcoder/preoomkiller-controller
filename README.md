# preoomkiller-controller

A controller to gracefully evict selected pods before they get **OOMKilled** by
**Kubernetes**.  
Usefull to workaround memory leaks.

# How it works?

`preoomkiller-controller` watches (once every `60s` by default) memory usage
metrics for all pods matching label selector `preoomkiller-enabled=true`.
Pods can specify a **preoomkiller** `memory-threshold`, e.g., `512Mi`, `1Gi`, etc.
via an annotation `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`.
When `preoomkiller-controller` finds that the pods' memory usage has crossed
the specified threshold, it starts trying to evict the pod, until it's evicted.

This operation is very safe, as it uses Kubernetes' pod **eviction** API to
evict pods. Pod eviction API takes into account **PodDisruptionBudget** for
the pods and ensures that a specified minimum number of ready pods are always
available.

# Usage

## Deploy

### Setup RBAC

```
kubectl -n kube-system create serviceaccount preoomkiller-controller

cat <<EOF | kubectl apply -f -
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: preoomkiller-controller
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - metrics.k8s.io
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods/eviction
  verbs:
  - create
EOF

kubectl create clusterrolebinding preoomkiller-controller --clusterrole=preoomkiller-controller --serviceaccount=kube-system:preoomkiller-controller
```

### Deploy controller

```
kubectl -n kube-system run --image=gha.io/skillcoder/preoomkiller-controller:latest --serviceaccount=preoomkiller-controller
```

### Add labels, annotations to pods

You can configure pods, deployments, statefulsets, daemonsets to add:

- Pod label `preoomkiller-enabled: true`
- Pod annotation: `preoomkiller.beta.k8s.skillcoder.com/memory-threshold: 1250Mi`

For example:

```
apiVersion: v1
kind: Pod
metadata:
  name: annotations-demo
  labels:
    preoomkiller-enabled: true
  annotations:
    imageregistry: "https://hub.docker.com/"
    preoomkiller.beta.k8s.skillcoder.com/memory-threshold: 2Gi
spec:
  containers:
  - name: nginx
    image: nginx:1.29.4-alpine3.23-slim
    ports:
    - containerPort: 80
 ```
