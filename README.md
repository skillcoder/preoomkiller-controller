# preoomkiller-controller

A Kubernetes controller that gracefully evicts selected pods before they get **OOMKilled** by Kubernetes. Useful for working around memory leaks.

## Compatibility with upstream

Intentionally **NOT compatible** with upstream:
- Uses environment variables for configuration instead of flags
- Different default label: `preoomkiller.beta.k8s.skillcoder.com/enabled=true`
- Different default annotation: `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`
- Different default interval: `300s` instead of `60s`

You can match upstream behavior by setting:

- **`PREOOMKILLER_POD_LABEL_SELECTOR`** — label selector to list pods (default: `preoomkiller.beta.k8s.skillcoder.com/enabled=true`)
- **`PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD`** — annotation key for memory threshold (default: `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`)

### Using upstream label and annotation

To use the same Pod label and annotation as upstream, set the controller env in your Deployment (or similar) manifest:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: preoomkiller-controller
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: preoomkiller-controller
  template:
    metadata:
      labels:
        app: preoomkiller-controller
    spec:
      serviceAccountName: preoomkiller-controller
      containers:
      - name: controller
        image: gha.io/skillcoder/preoomkiller-controller:latest
        env:
        - name: INTERVAL
          value: "60"
        - name: PREOOMKILLER_POD_LABEL_SELECTOR
          value: "preoomkiller-enabled=true"
        - name: PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD
          value: "preoomkiller.alpha.k8s.zapier.com/memory-threshold"
```

Then add the upstream label and annotation to your pod template:

- **Label:** `preoomkiller-enabled: "true"`
- **Annotation:** `preoomkiller.alpha.k8s.zapier.com/memory-threshold: "2Gi"` (or your desired threshold)

Example Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: upstream-demo
  labels:
    preoomkiller-enabled: "true"
  annotations:
    preoomkiller.alpha.k8s.zapier.com/memory-threshold: "2Gi"
spec:
  containers:
  - name: app
    image: your-image:latest
```

## How it works

The `preoomkiller-controller` watches memory usage metrics for all pods matching the label selector `preoomkiller.beta.k8s.skillcoder.com/enabled=true`. By default, it checks at most once every `300s`, with a 1 second delay between each pod.

Pods can specify a memory threshold (e.g., `512Mi`, `1Gi`) via the annotation `preoomkiller.beta.k8s.skillcoder.com/memory-threshold`. When the controller detects that a pod's memory usage has crossed the specified threshold, it attempts to evict the pod using Kubernetes' eviction API until the pod is successfully evicted.

> **Important:** The threshold in the annotation applies to the **sum of all container memory usages** in the pod, including sidecars.

This operation is safe because it uses Kubernetes' pod **eviction** API, which respects **PodDisruptionBudget** constraints and ensures that a specified minimum number of ready pods remain available.

## Usage

### Environment variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `KUBECONFIG` | (empty) | Path to kubeconfig file. |
| `KUBERNETES_MASTER` | (empty) | Kubernetes API server URL. |
| `LOG_LEVEL` | `info` | Log level (e.g. `debug`, `info`, `warn`, `error`). |
| `LOG_FORMAT` | `json` | Log format (`json` or `text`). |
| `HTTP_PORT` | `8080` | Port for health/readiness HTTP server. |
| `INTERVAL` | `300` | Reconciliation interval in seconds. |
| `PINGER_INTERVAL` | `10` | Pinger check interval in seconds. |
| `PREOOMKILLER_POD_LABEL_SELECTOR` | `preoomkiller.beta.k8s.skillcoder.com/enabled=true` | Label selector to list pods. |
| `PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD` | `preoomkiller.beta.k8s.skillcoder.com/memory-threshold` | Annotation key read from pod metadata for the memory threshold. See below for value format. |
| `PREOOMKILLER_ANNOTATION_RESTART_SCHEDULE` | `preoomkiller.beta.k8s.skillcoder.com/restart-schedule` | Annotation key for scheduled restart cron. |
| `PREOOMKILLER_ANNOTATION_TZ` | `preoomkiller.beta.k8s.skillcoder.com/tz` | Annotation key for schedule timezone. |
| `PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX` | `30` | Max jitter in seconds for scheduled eviction. |

**Memory threshold annotation value** (the value pods set on the annotation key above):

- **Absolute:** Kubernetes quantity string, e.g. `512Mi`, `1Gi`. Eviction when pod memory usage exceeds this amount.
- **Percentage:** Number followed by `%`, e.g. `80%`, `50%`. Value must be in (0, 100]. Interpreted as a percentage of the pod’s total memory limit (sum of all container limits). If the pod has no memory limit, percentage thresholds are ignored and the pod is not evicted.

### Scheduled pod restart (restart-schedule)

To mitigate slow memory leaks without waiting for OOM, you can schedule restarts during low-usage hours. Pods may have only `restart-schedule`, only `memory-threshold`, or both.

**Annotations** (on the pod template):

- **`preoomkiller.beta.k8s.skillcoder.com/restart-schedule`** — Standard 5-field cron (minute-first), e.g. `"40 7 * * *"` (daily at 07:40 in the configured timezone).
- **`preoomkiller.beta.k8s.skillcoder.com/tz`** — Optional IANA timezone for the schedule (e.g. `"America/New_York"`). Defaults to UTC. Ignored when the schedule uses inline `CRON_TZ=`.

Inline timezone in the schedule is also supported: `"CRON_TZ=America/New_York 0 6 * * *"`.

The controller writes a **`preoomkiller.beta.k8s.skillcoder.com/restart-at`** annotation (ISO 8601 timestamp) to the pod when it schedules a restart. Do not set this annotation manually; it is managed by the controller and disappears when the pod is evicted and recreated.

Eviction runs at the scheduled time plus a random jitter (see `PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX`). If the controller was down at the scheduled time, it detects missed evictions and evicts on the next reconcile.

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
  - patch
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
