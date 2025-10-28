# Installation Guide

This guide will walk you through installing glua-webhook in your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster 1.20 or later
- `kubectl` configured to access your cluster
- Cluster admin permissions
- TLS certificates for webhook (we'll show you how to generate them)

## Installation Methods

### Method 1: Using Kind (Local Development)

The easiest way to get started is using Kind:

```bash
# Clone the repository
git clone https://github.com/thomas-maurice/glua-webhook
cd glua-webhook

# Create Kind cluster and deploy
make kind-create
make kind-deploy

# Verify deployment
kubectl get pods -n glua-webhook
```

### Method 2: Using Pre-built Manifests

#### Step 1: Generate TLS Certificates

```bash
# Create certificate directory
mkdir -p certs

# Generate CA
openssl genrsa -out certs/ca.key 2048
openssl req -x509 -new -nodes -key certs/ca.key \
  -subj "/CN=glua-webhook-ca" \
  -days 10000 \
  -out certs/ca.crt

# Generate webhook certificate
openssl genrsa -out certs/tls.key 2048
openssl req -new -key certs/tls.key \
  -subj "/CN=glua-webhook.glua-webhook.svc" \
  -out certs/tls.csr

# Sign certificate
openssl x509 -req -in certs/tls.csr \
  -CA certs/ca.crt -CAkey certs/ca.key \
  -CAcreateserial -out certs/tls.crt -days 10000 \
  -extensions v3_req -extfile <(cat <<EOF
[v3_req]
subjectAltName = DNS:glua-webhook.glua-webhook.svc,DNS:glua-webhook.glua-webhook.svc.cluster.local
EOF
)
```

#### Step 2: Create Kubernetes Secret

```bash
kubectl create namespace glua-webhook
kubectl create secret tls glua-webhook-certs \
  --cert=certs/tls.crt \
  --key=certs/tls.key \
  -n glua-webhook
```

#### Step 3: Update Webhook Configurations

Get the CA bundle:

```bash
CA_BUNDLE=$(cat certs/ca.crt | base64 | tr -d '\n')
```

Update webhook configurations:

```bash
# Update mutating webhook
kubectl get mutatingwebhookconfiguration glua-mutating-webhook -o yaml | \
  sed "s/caBundle: .*/caBundle: $CA_BUNDLE/" | \
  kubectl apply -f -

# Update validating webhook
kubectl get validatingwebhookconfiguration glua-validating-webhook -o yaml | \
  sed "s/caBundle: .*/caBundle: $CA_BUNDLE/" | \
  kubectl apply -f -
```

#### Step 4: Deploy Webhook

```bash
# Apply all manifests
kubectl apply -f examples/manifests/00-namespace.yaml
kubectl apply -f examples/manifests/04-rbac.yaml
kubectl apply -f examples/manifests/02-deployment.yaml
kubectl apply -f examples/manifests/03-service.yaml
kubectl apply -f examples/manifests/05-mutating-webhook.yaml
kubectl apply -f examples/manifests/06-validating-webhook.yaml
```

#### Step 5: Verify Installation

```bash
# Check webhook pods
kubectl get pods -n glua-webhook

# Check webhook logs
kubectl logs -n glua-webhook deployment/glua-webhook

# Test webhook endpoints
kubectl run test-pod --image=nginx --dry-run=server -o yaml
```

### Method 3: Using Helm (Coming Soon)

Helm chart support is coming soon!

## Post-Installation

### Enable Webhook for Namespaces

The webhook only processes resources in namespaces with specific labels:

```bash
# Enable mutating webhook
kubectl label namespace default glua.maurice.fr/enabled=true

# Enable validating webhook
kubectl label namespace default glua.maurice.fr/validation-enabled=true
```

### Create Your First Script

Create a ConfigMap with a Lua script:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-first-script
  namespace: default
data:
  script.lua: |
    -- Add a label
    if object.metadata.labels == nil then
      object.metadata.labels = {}
    end
    object.metadata.labels["processed-by"] = "glua-webhook"
```

Apply it:

```bash
kubectl apply -f my-first-script.yaml
```

### Test Your Script

Create a pod that uses your script:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations:
    glua.maurice.fr/scripts: "default/my-first-script"
spec:
  containers:
  - name: nginx
    image: nginx:latest
```

Check the result:

```bash
kubectl get pod test-pod -o jsonpath='{.metadata.labels}'
```

You should see the `processed-by` label!

## Troubleshooting

### Webhook Not Working

1. **Check webhook pods are running**:
   ```bash
   kubectl get pods -n glua-webhook
   ```

2. **Check webhook logs**:
   ```bash
   kubectl logs -n glua-webhook deployment/glua-webhook
   ```

3. **Verify namespace labels**:
   ```bash
   kubectl get namespace default -o yaml | grep glua
   ```

4. **Check webhook configuration**:
   ```bash
   kubectl get mutatingwebhookconfiguration glua-mutating-webhook -o yaml
   ```

### Certificate Issues

If you see TLS errors:

1. Verify secret exists:
   ```bash
   kubectl get secret glua-webhook-certs -n glua-webhook
   ```

2. Check certificate validity:
   ```bash
   kubectl get secret glua-webhook-certs -n glua-webhook -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text
   ```

### Script Not Executing

1. **Check ConfigMap exists**:
   ```bash
   kubectl get cm my-first-script -n default
   ```

2. **Verify annotation format**:
   ```yaml
   annotations:
     glua.maurice.fr/scripts: "namespace/configmap-name"
   ```

3. **Check webhook logs for errors**:
   ```bash
   kubectl logs -n glua-webhook deployment/glua-webhook | grep ERROR
   ```

## Uninstallation

To remove glua-webhook:

```bash
# Delete webhook configurations
kubectl delete mutatingwebhookconfiguration glua-mutating-webhook
kubectl delete validatingwebhookconfiguration glua-validating-webhook

# Delete webhook deployment
kubectl delete namespace glua-webhook

# For Kind clusters
make kind-delete
```

## Next Steps

- [Writing Lua Scripts](../guides/writing-scripts.md)
- [Examples](../examples/index.md)
- [Configuration Reference](../reference/configuration.md)
