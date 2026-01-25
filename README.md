# preoomkiller-controller

A Kubernetes controller that gracefully evicts selected pods before they get **OOMKilled** by Kubernetes. Useful for working around memory leaks.

## Compatibility with upstream

Intentionally **NOT compatible** with upstream:
- Uses environment variables for configuration instead of flags
- Different label: `preoomkiller.beta.k8s.skillcoder.com/enabled=true`
- Different annotation: `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`
- Different default interval: `300s` instead of `60s`

## How it works

The `preoomkiller-controller` watches memory usage metrics for all pods matching the label selector `preoomkiller.beta.k8s.skillcoder.com/enabled=true`. By default, it checks at most once every `300s`, with a 1 second delay between each pod.

Pods can specify a memory threshold (e.g., `512Mi`, `1Gi`) via the annotation `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`. When the controller detects that a pod's memory usage has crossed the specified threshold, it attempts to evict the pod using Kubernetes' eviction API until the pod is successfully evicted.

> **Important:** The threshold in the annotation applies to the **sum of all container memory usages** in the pod, including sidecars.

This operation is safe because it uses Kubernetes' pod **eviction** API, which respects **PodDisruptionBudget** constraints and ensures that a specified minimum number of ready pods remain available.

## Usage

### Deployment

#### Setup RBAC

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

kubectl create clusterrolebinding preoomkiller-controller \
  --clusterrole=preoomkiller-controller \
  --serviceaccount=kube-system:preoomkiller-controller
```

#### Deploy controller

```bash
kubectl -n kube-system run preoomkiller-controller \
  --image=gha.io/skillcoder/preoomkiller-controller:latest \
  --serviceaccount=preoomkiller-controller \
  --restart=Always
```

#### Configure pods

Add the following to your pods templates metadata (not to the deployments, statefulsets, or daemonsets metadata):

- **Label:** `preoomkiller.beta.k8s.skillcoder.com/enabled: "true"`
- **Annotation:** `preoomkiller.beta.k8s.skillcoder.com/memory-threshold: "1250Mi"`

Example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: annotations-demo
  labels:
    preoomkiller.beta.k8s.skillcoder.com/enabled: "true"
  annotations:
    preoomkiller.beta.k8s.skillcoder.com/memory-threshold: "2Gi"
spec:
  containers:
  - name: nginx
    image: nginx:1.29.4-alpine3.23-slim
    ports:
    - containerPort: 80
```
