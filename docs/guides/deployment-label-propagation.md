# Label Propagation from Deployments to Pods

This guide shows how to deploy glua-webhook and use it to propagate labels from Deployments to Pods with automatic value modification.

## Use Case

You have a Deployment with label:
```yaml
foo.bar/baz: "hello=true"
```

You want Pods created by this Deployment to automatically get:
```yaml
foo.bar/baz: "hello=true-pod"
```

This is useful for:
- Environment-specific labeling
- Cost allocation tracking
- Custom monitoring labels
- Multi-tenant resource tagging

## Prerequisites

- Kubernetes cluster (1.20+)
- `kubectl` configured to access the cluster
- Cluster admin permissions (for creating webhooks)

## Step 1: Build the glua-webhook Binary

```bash
# Clone the repository
git clone https://github.com/thomas-maurice/glua-webhook
cd glua-webhook

# Build the binary (using Nix)
nix develop --command make build

# Or without Nix
make build
```

This creates `bin/glua-webhook`.

## Step 2: Create TLS Certificates

Kubernetes requires webhooks to use TLS. Generate certificates:

```bash
# Create certificate directory
mkdir -p certs

# Generate CA
openssl genrsa -out certs/ca.key 2048
openssl req -x509 -new -nodes -key certs/ca.key \
  -subj "/CN=glua-webhook-ca" \
  -days 3650 \
  -out certs/ca.crt

# Generate webhook certificate
openssl genrsa -out certs/tls.key 2048
openssl req -new -key certs/tls.key \
  -subj "/CN=glua-webhook.glua-webhook.svc" \
  -out certs/tls.csr

# Sign certificate with SAN for webhook service
openssl x509 -req -in certs/tls.csr \
  -CA certs/ca.crt -CAkey certs/ca.key \
  -CAcreateserial -out certs/tls.crt -days 3650 \
  -extensions v3_req -extfile <(cat <<EOF
[v3_req]
subjectAltName = DNS:glua-webhook.glua-webhook.svc,DNS:glua-webhook.glua-webhook.svc.cluster.local
EOF
)
```

## Step 3: Create Namespace and Deploy Webhook

```bash
# Create namespace
kubectl create namespace glua-webhook

# Create TLS secret
kubectl create secret tls glua-webhook-certs \
  --cert=certs/tls.crt \
  --key=certs/tls.key \
  -n glua-webhook

# Create RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: glua-webhook
  namespace: glua-webhook
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: glua-webhook
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: glua-webhook
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: glua-webhook
subjects:
- kind: ServiceAccount
  name: glua-webhook
  namespace: glua-webhook
EOF

# Deploy the webhook (using the binary directly)
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: glua-webhook
  namespace: glua-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: glua-webhook
  template:
    metadata:
      labels:
        app: glua-webhook
    spec:
      serviceAccountName: glua-webhook
      containers:
      - name: webhook
        image: glua-webhook:latest  # Build with: make docker-build
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8443
          name: webhook
        volumeMounts:
        - name: certs
          mountPath: /etc/webhook/certs
          readOnly: true
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: certs
        secret:
          secretName: glua-webhook-certs
---
apiVersion: v1
kind: Service
metadata:
  name: glua-webhook
  namespace: glua-webhook
spec:
  selector:
    app: glua-webhook
  ports:
  - port: 443
    targetPort: 8443
    protocol: TCP
EOF
```

**Note:** You need to build the Docker image first:
```bash
make docker-build
# If using kind: kind load docker-image glua-webhook:latest
```

## Step 4: Create the Lua Script ConfigMap

Create a ConfigMap with the label propagation script:

```bash
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: propagate-deployment-labels
  namespace: default
data:
  script.lua: |
    -- Propagate labels from Deployment to Pod with -pod suffix
    local log = require("log")

    -- Only process Pods
    if object.kind ~= "Pod" then
      log.info("Skipping non-Pod resource: " .. object.kind)
      return
    end

    -- Ensure labels table exists
    if object.metadata.labels == nil then
      object.metadata.labels = {}
    end

    -- Find labels matching foo.bar/baz and add -pod suffix
    for key, value in pairs(object.metadata.labels) do
      if string.match(key, "^foo%.bar/baz") then
        local new_value = value .. "-pod"
        if object.metadata.labels[key] ~= new_value then
          log.info("Propagating label " .. key .. ": " .. value .. " -> " .. new_value)
          object.metadata.labels[key] = new_value
        end
      end
    end
EOF
```

## Step 5: Configure the MutatingWebhookConfiguration

```bash
# Get CA bundle from certificate
CA_BUNDLE=$(cat certs/ca.crt | base64 | tr -d '\n')

# Create webhook configuration
kubectl apply -f - <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: glua-mutating-webhook
webhooks:
- name: mutate.glua.maurice.fr
  clientConfig:
    service:
      name: glua-webhook
      namespace: glua-webhook
      path: /mutate
    caBundle: ${CA_BUNDLE}
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments"]
  - operations: ["CREATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
  namespaceSelector:
    matchLabels:
      glua.maurice.fr/enabled: "true"
  admissionReviewVersions: ["v1", "v1beta1"]
  sideEffects: None
  failurePolicy: Ignore
EOF
```

## Step 6: Enable the Webhook for Your Namespace

```bash
# Enable webhook for default namespace
kubectl label namespace default glua.maurice.fr/enabled=true
```

## Step 7: Create a Deployment with Labels

Now create a Deployment with the `foo.bar/baz` label:

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
  labels:
    foo.bar/baz: "hello=true"
  annotations:
    glua.maurice.fr/scripts: "default/propagate-deployment-labels"
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
        foo.bar/baz: "hello=true"  # This label will be propagated to Pods
      annotations:
        glua.maurice.fr/scripts: "default/propagate-deployment-labels"
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
EOF
```

**Important Notes:**
- The Deployment has label `foo.bar/baz: "hello=true"`
- The Pod template includes the same label
- The annotation `glua.maurice.fr/scripts` references our ConfigMap
- The webhook will modify the Pod label value to `hello=true-pod`

## Step 8: Verify the Result

Check that Pods have the modified label:

```bash
# Get Pods
kubectl get pods -l app=test-app

# Check labels on a Pod
kubectl get pod -l app=test-app -o yaml | grep -A 5 "labels:"
```

You should see:
```yaml
  labels:
    app: test-app
    foo.bar/baz: hello=true-pod  # ← Modified by webhook!
    pod-template-hash: xxxxx
```

Compare with the Deployment:
```bash
kubectl get deployment test-app -o yaml | grep -A 3 "labels:"
```

Deployment has:
```yaml
  labels:
    foo.bar/baz: hello=true  # ← Original value
```

Pods have:
```yaml
  labels:
    foo.bar/baz: hello=true-pod  # ← Modified value!
```

## Step 9: Check Webhook Logs

View logs to see the webhook in action:

```bash
kubectl logs -n glua-webhook deployment/glua-webhook -f
```

You should see:
```
INFO  Processing mutating admission request: Kind=Pod, Namespace=default, Name=test-app-xxx
INFO  Found scripts annotation: default/propagate-deployment-labels
INFO  Executing script default/propagate-deployment-labels
INFO  Propagating label foo.bar/baz: hello=true -> hello=true-pod
INFO  Admission allowed with modifications
```

## Testing Locally Before Deployment

Before deploying to your cluster, test the script locally:

```bash
# Create a test Pod JSON
cat > test-pod.json <<EOF
{
  "kind": "Pod",
  "metadata": {
    "name": "test-pod",
    "namespace": "default",
    "labels": {
      "app": "test-app",
      "foo.bar/baz": "hello=true"
    }
  },
  "spec": {
    "containers": [
      {
        "name": "nginx",
        "image": "nginx:latest"
      }
    ]
  }
}
EOF

# Test the script locally
cat test-pod.json | ./bin/glua-webhook exec \
  --script examples/scripts/propagate-deployment-labels.lua
```

Expected output:
```json
{
  "kind": "Pod",
  "metadata": {
    "labels": {
      "app": "test-app",
      "foo.bar/baz": "hello=true-pod"
    },
    "name": "test-pod",
    "namespace": "default"
  },
  "spec": {
    "containers": [
      {
        "image": "nginx:latest",
        "name": "nginx"
      }
    ]
  }
}
```

Notice the label value changed from `hello=true` to `hello=true-pod`!

## Customizing the Script

You can modify the script to:

1. **Change the label pattern:**
   ```lua
   if string.match(key, "^your-prefix/") then
   ```

2. **Change the suffix:**
   ```lua
   local new_value = value .. "-custom-suffix"
   ```

3. **Apply different transformations:**
   ```lua
   -- Convert to uppercase
   local new_value = string.upper(value)

   -- Add prefix instead of suffix
   local new_value = "pod-" .. value

   -- Replace part of the value
   local new_value = string.gsub(value, "=true", "=enabled")
   ```

4. **Conditional logic:**
   ```lua
   -- Only for production namespace
   if object.metadata.namespace == "production" then
     -- Apply transformation
   end
   ```

## Troubleshooting

### Webhook Not Working

1. **Check webhook pods are running:**
   ```bash
   kubectl get pods -n glua-webhook
   ```

2. **Check webhook logs for errors:**
   ```bash
   kubectl logs -n glua-webhook deployment/glua-webhook
   ```

3. **Verify namespace label:**
   ```bash
   kubectl get namespace default -o jsonpath='{.metadata.labels}'
   ```
   Should show `glua.maurice.fr/enabled: "true"`

4. **Check webhook configuration:**
   ```bash
   kubectl get mutatingwebhookconfiguration glua-mutating-webhook -o yaml
   ```

### Labels Not Being Modified

1. **Check annotation on Pod template:**
   ```bash
   kubectl get deployment test-app -o yaml | grep glua.maurice.fr/scripts
   ```

2. **Verify ConfigMap exists:**
   ```bash
   kubectl get cm propagate-deployment-labels -n default
   ```

3. **Test script locally first:**
   ```bash
   kubectl get pod <pod-name> -o json | \
     ./bin/glua-webhook exec --script examples/scripts/propagate-deployment-labels.lua
   ```

### Certificate Issues

If you see TLS errors:

```bash
# Regenerate certificates with correct SANs
# Then update the secret
kubectl delete secret glua-webhook-certs -n glua-webhook
kubectl create secret tls glua-webhook-certs \
  --cert=certs/tls.crt \
  --key=certs/tls.key \
  -n glua-webhook

# Restart webhook pods
kubectl rollout restart deployment glua-webhook -n glua-webhook
```

## Cleanup

To remove everything:

```bash
# Delete the Deployment
kubectl delete deployment test-app -n default

# Delete the ConfigMap
kubectl delete cm propagate-deployment-labels -n default

# Delete webhook configuration
kubectl delete mutatingwebhookconfiguration glua-mutating-webhook

# Delete webhook deployment
kubectl delete namespace glua-webhook

# Remove namespace label
kubectl label namespace default glua.maurice.fr/enabled-
```

## Next Steps

- [Writing More Complex Scripts](writing-scripts.md)
- [Validation Webhooks](validation-webhooks.md)
- [Multiple Script Chaining](script-chaining.md)
- [Production Deployment Best Practices](production-deployment.md)

## Summary

You've successfully:
1. ✅ Deployed glua-webhook to your cluster
2. ✅ Created a Lua script that propagates labels with modifications
3. ✅ Configured the webhook to process Deployments and Pods
4. ✅ Verified that Pods get labels modified automatically
5. ✅ Learned how to test scripts locally before deployment

**Result:** Any Deployment with `foo.bar/baz: "hello=true"` will create Pods with `foo.bar/baz: "hello=true-pod"` automatically!
