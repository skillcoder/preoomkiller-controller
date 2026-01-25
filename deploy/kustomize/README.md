# Kustomize Manifests

This directory contains kustomize manifests for deploying the preoomkiller-controller to Kubernetes.

## Structure

```
kustomize/
├── base/                    # Base resources
│   ├── kustomization.yaml
│   ├── serviceaccount.yaml
│   ├── clusterrole.yaml
│   ├── clusterrolebinding.yaml
│   └── deployment.yaml
└── overlays/
    └── production/          # Production overlay
        └── kustomization.yaml
```

## Usage

### Deploy base configuration

```bash
kubectl apply -k deploy/kustomize/base
```

### Deploy with production overlay

```bash
kubectl apply -k deploy/kustomize/overlays/production
```

### Customize image tag for production

Edit `deploy/kustomize/overlays/production/kustomization.yaml` and update the `images` section:

```yaml
images:
- name: skillcoder/preoomkiller-controller
  newTag: v1.0.0  # or your desired tag
```

Then apply:

```bash
kubectl apply -k deploy/kustomize/overlays/production
```

### Preview changes before applying

```bash
kubectl kustomize deploy/kustomize/base
kubectl kustomize deploy/kustomize/overlays/production
```

## Resources

The base kustomization includes:
- **ServiceAccount**: `preoomkiller-controller` in `kube-system` namespace
- **ClusterRole**: Permissions for pods, metrics, and evictions
- **ClusterRoleBinding**: Binds the ServiceAccount to the ClusterRole
- **Deployment**: Controller deployment with 1 replica
