# preoomkiller-controller

A controller to gracefully evict selected pods before they get **OOMKilled** by
**Kubernetes**.  
Usefull to workaround memory leaks.

# Compatibility with upstream

Intentionally NOT compatible:  
- ENV for config insted of flags
- different label
- different annotation
- different default interval (300s insted 60s)

# How it works?

`preoomkiller-controller` watches (at most once every `300s` by default, with 1 second delay between each pod) memory usage
metrics for all pods matching label selector `preoomkiller.beta.k8s.skillcoder.com/enabled=true`.
Pods can specify a **preoomkiller** `memory-threshold`, e.g., `512Mi`, `1Gi`, etc.
via an annotation `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`.
When `preoomkiller-controller` finds that the pods' memory usage has crossed
the specified threshold, it starts trying to evict the pod, until it's evicted.

IMPORTANT! Keep in mind threshold in annotation applied to sum of all containers memory usages in the pod, including sidecars.

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

- Pod label `preoomkiller.beta.k8s.skillcoder.com/enabled: true`
- Pod annotation: `preoomkiller.beta.k8s.skillcoder.com/memory-threshold: 1250Mi`

For example:

```
apiVersion: v1
kind: Pod
metadata:
  name: annotations-demo
  labels:
    preoomkiller.beta.k8s.skillcoder.com/enabled: true
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
