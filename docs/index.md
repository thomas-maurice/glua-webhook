# glua-webhook Documentation

Welcome to the glua-webhook documentation! This webhook allows you to dynamically mutate and validate Kubernetes resources using Lua scripts stored in ConfigMaps.

## What is glua-webhook?

glua-webhook is a Kubernetes admission webhook that:

- **Processes Kubernetes resources** using Lua scripts
- **Stores scripts in ConfigMaps** for easy management
- **Supports both mutation and validation** of resources
- **Provides full access to glua modules** (json, yaml, http, time, etc.)
- **Executes scripts sequentially** with isolated VM instances

## Quick Links

- [Getting Started](getting-started/installation.md)
- [Writing Lua Scripts](guides/writing-scripts.md)
- [Examples](examples/index.md)
- [API Reference](reference/annotations.md)

## Features

### Dual Webhook Support

Support for both `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook` configurations.

### ConfigMap-based Scripts

Store your Lua scripts in ConfigMaps and reference them via annotations:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    glua.maurice.fr/scripts: "default/my-script"
```

### Glua Module Ecosystem

Access to all glua modules:

- **json**: JSON encoding/decoding
- **yaml**: YAML processing
- **http**: HTTP client
- **time**: Time manipulation
- **hash**: Hashing functions
- **base64**: Base64 encoding
- **template**: Template processing
- And more!

### Comprehensive Logging

Detailed logging at every step for easy debugging and monitoring.

## Architecture

```
┌─────────────────────────────────────────┐
│          Kubernetes API Server          │
└──────────────────┬──────────────────────┘
                   │
                   │ Admission Request
                   ▼
┌─────────────────────────────────────────┐
│            glua-webhook                  │
│                                          │
│  ┌──────────────────────────────────┐   │
│  │ 1. Parse Admission Request       │   │
│  │ 2. Extract Annotations           │   │
│  │ 3. Load ConfigMap Scripts        │   │
│  │ 4. Execute Lua Scripts (α order) │   │
│  │ 5. Generate JSON Patch           │   │
│  │ 6. Return Admission Response     │   │
│  └──────────────────────────────────┘   │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│         ConfigMaps (Lua Scripts)        │
└─────────────────────────────────────────┘
```

## Use Cases

### Resource Mutation

- Inject sidecar containers
- Add labels and annotations
- Set default values
- Modify resource requests/limits

### Resource Validation

- Enforce naming conventions
- Validate label presence
- Check security policies
- Enforce organizational standards

### Dynamic Policy Enforcement

- Update policies without redeploying
- Environment-specific logic
- Complex conditional mutations
- Multi-step transformations

## Community

- **GitHub**: [glua-webhook](https://github.com/thomas-maurice/glua-webhook)
- **Issues**: [Report bugs](https://github.com/thomas-maurice/glua-webhook/issues)
- **Discussions**: [Ask questions](https://github.com/thomas-maurice/glua-webhook/discussions)

## License

[Add your license information]

## Next Steps

Ready to get started? Check out our [Installation Guide](getting-started/installation.md)!
