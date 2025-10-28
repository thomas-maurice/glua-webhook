# Complete Webhook Example

Production-ready example webhook server demonstrating all features of glua-webhook.

## Features

- Full HTTPS server with TLS 1.2+
- Mutating and validating admission webhooks
- Health checks (liveness and readiness probes)
- Graceful shutdown with configurable timeout
- Comprehensive error handling and logging
- Production-ready HTTP server settings
- Complete test coverage with benchmarks
- Kubernetes in-cluster and kubeconfig support

## Quick Start

### Build

```bash
go build -o webhook-example examples/webhook/main.go
```

### Run Locally

```bash
# Generate self-signed certificate for testing
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"

# Run webhook
./webhook-example \
  --port 8443 \
  --cert cert.pem \
  --key key.pem \
  --kubeconfig ~/.kube/config
```

### Run in Kubernetes

```bash
# Deploy webhook
kubectl apply -f examples/webhook/deployment.yaml

# Check status
kubectl get pods -n glua-webhook
kubectl logs -n glua-webhook deployment/glua-webhook -f
```

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8443` | HTTPS server port |
| `--cert` | `/etc/webhook/certs/tls.crt` | TLS certificate file path |
| `--key` | `/etc/webhook/certs/tls.key` | TLS private key file path |
| `--kubeconfig` | `""` | Path to kubeconfig (empty = in-cluster) |
| `--mutating-path` | `/mutate` | Mutating webhook endpoint path |
| `--validating-path` | `/validate` | Validating webhook endpoint path |
| `--shutdown-timeout` | `30` | Graceful shutdown timeout (seconds) |
| `--read-timeout` | `15` | HTTP read timeout (seconds) |
| `--write-timeout` | `15` | HTTP write timeout (seconds) |
| `--max-header-bytes` | `1048576` | Maximum HTTP header size (bytes) |
| `--enable-validation` | `true` | Enable validating webhook endpoint |

### Environment Variables

The webhook can also be configured via environment variables in Kubernetes:

```yaml
env:
- name: PORT
  value: "8443"
- name: SHUTDOWN_TIMEOUT
  value: "30"
```

## Endpoints

### POST /mutate

Mutating admission webhook endpoint.

**Request:** AdmissionReview (admission.k8s.io/v1)
**Response:** AdmissionResponse with JSONPatch

**Example:**

```bash
curl -k https://localhost:8443/mutate \
  -H "Content-Type: application/json" \
  -d @admission-review.json
```

### POST /validate

Validating admission webhook endpoint.

**Request:** AdmissionReview (admission.k8s.io/v1)
**Response:** AdmissionResponse with allowed/denied status

### GET /healthz

Liveness probe endpoint. Always returns 200 OK if server is running.

**Example:**

```bash
curl -k https://localhost:8443/healthz
# Output: ok
```

### GET /readyz

Readiness probe endpoint. Returns 200 OK if Kubernetes API is accessible, 503 otherwise.

**Example:**

```bash
curl -k https://localhost:8443/readyz
# Output: ready
```

## TLS Configuration

### Generate Certificates

#### For Testing (Self-Signed)

```bash
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/CN=webhook.glua-webhook.svc"
```

#### For Production

Use cert-manager or your organization's PKI:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: glua-webhook-cert
  namespace: glua-webhook
spec:
  secretName: glua-webhook-certs
  dnsNames:
  - glua-webhook.glua-webhook.svc
  - glua-webhook.glua-webhook.svc.cluster.local
  issuerRef:
    name: ca-issuer
    kind: ClusterIssuer
```

### TLS Security

The webhook enforces:
- TLS 1.2 minimum
- Strong cipher suites only:
  - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
  - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384

## Testing

### Run Tests

```bash
cd examples/webhook
go test -v -race -cover
```

### Run Benchmarks

```bash
go test -bench=. -benchmem
```

### Test Coverage

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Example Test Output

```
=== RUN   TestHealthzHandler
--- PASS: TestHealthzHandler (0.00s)
=== RUN   TestReadyzHandler
--- PASS: TestReadyzHandler (0.00s)
=== RUN   TestLoadTLSConfig
--- PASS: TestLoadTLSConfig (0.01s)
=== RUN   TestServerConfiguration
--- PASS: TestServerConfiguration (0.00s)
PASS
coverage: 85.2% of statements
```

## Production Deployment

### Deployment Manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: glua-webhook
  namespace: glua-webhook
spec:
  replicas: 2
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
        image: your-registry/glua-webhook:latest
        args:
        - --port=8443
        - --cert=/etc/webhook/certs/tls.crt
        - --key=/etc/webhook/certs/tls.key
        - --shutdown-timeout=30
        ports:
        - containerPort: 8443
          name: webhook
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        volumeMounts:
        - name: certs
          mountPath: /etc/webhook/certs
          readOnly: true
      volumes:
      - name: certs
        secret:
          secretName: glua-webhook-certs
```

### Service Manifest

```yaml
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
```

### WebhookConfiguration

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: glua-webhook-mutating
webhooks:
- name: mutate.glua.maurice.fr
  clientConfig:
    service:
      name: glua-webhook
      namespace: glua-webhook
      path: /mutate
    caBundle: <base64-encoded-ca-cert>
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["*"]
    apiVersions: ["*"]
    resources: ["*"]
  namespaceSelector:
    matchLabels:
      glua.maurice.fr/enabled: "true"
  admissionReviewVersions: ["v1"]
  sideEffects: None
  timeoutSeconds: 10
  failurePolicy: Ignore
```

## Performance

### Benchmarks

```
BenchmarkHealthzHandler-8    10000000    125 ns/op    0 B/op    0 allocs/op
BenchmarkReadyzHandler-8      1000000   1523 ns/op  896 B/op   18 allocs/op
```

### Recommendations

- **CPU**: 100m request, 500m limit per replica
- **Memory**: 128Mi request, 512Mi limit per replica
- **Replicas**: Minimum 2 for high availability
- **Timeouts**:
  - Read: 15s
  - Write: 15s
  - Shutdown: 30s
  - Webhook: 10s (Kubernetes default)

## Troubleshooting

### Common Issues

#### Certificate Errors

```bash
# Check certificate validity
openssl x509 -in cert.pem -text -noout

# Check certificate matches key
openssl x509 -noout -modulus -in cert.pem | openssl md5
openssl rsa -noout -modulus -in key.pem | openssl md5
```

#### Connection Refused

```bash
# Check if webhook is listening
netstat -tuln | grep 8443

# Check webhook logs
kubectl logs -n glua-webhook deployment/glua-webhook

# Test connectivity from Pod
kubectl run test --rm -it --image=curlimages/curl -- \
  curl -k https://glua-webhook.glua-webhook.svc:443/healthz
```

#### Webhook Not Called

```bash
# Check namespace labels
kubectl get namespace default -o jsonpath='{.metadata.labels}'

# Check MutatingWebhookConfiguration
kubectl get mutatingwebhookconfiguration glua-webhook-mutating -o yaml

# Check webhook service
kubectl get svc -n glua-webhook
```

### Debug Logging

Enable verbose logging:

```go
logger := log.New(os.Stdout, "[webhook] ", log.LstdFlags|log.Lshortfile)
```

## Security Considerations

1. **TLS**: Always use valid TLS certificates in production
2. **RBAC**: Restrict webhook service account permissions
3. **Network Policies**: Limit webhook network access
4. **Resource Limits**: Prevent resource exhaustion
5. **Timeout**: Set reasonable timeouts to prevent DoS
6. **Validation**: Use failurePolicy=Fail for critical validations
7. **Audit**: Enable Kubernetes audit logging for webhook calls

## See Also

- [Main README](../../README.md)
- [Writing Lua Scripts](../../README.md#writing-lua-scripts)
- [Script Examples](../scripts/README.md)
- [Troubleshooting](../../README.md#troubleshooting)
