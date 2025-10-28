# Type Stubs and IDE Support

glua-webhook uses the [glua TypeRegistry](https://github.com/thomas-maurice/glua) to provide Lua Language Server (LSP) type annotations for Kubernetes resources. This enables IDE autocompletion and type checking for your Lua scripts.

## How It Works

The webhook automatically:

1. **Registers Kubernetes types** with the glua TypeRegistry
2. **Generates Lua annotations** for all Kubernetes API objects
3. **Provides type information** at runtime for better error messages

## Using TypeRegistry in Scripts

The TypeRegistry is used internally by the webhook to:

- Convert Kubernetes resources to Lua tables with type information
- Generate LSP annotations for IDE support
- Validate field access at runtime (optional)

### Type Information Available

For a Kubernetes Pod, the type information includes:

```lua
---@class corev1.Pod
---@field apiVersion string
---@field kind string
---@field metadata metav1.ObjectMeta
---@field spec corev1.PodSpec
---@field status corev1.PodStatus

---@class corev1.PodSpec
---@field containers corev1.Container[]
---@field volumes corev1.Volume[]
---@field nodeSelector table<string, string>
-- ... and more
```

## IDE Setup

### VSCode with Lua Language Server

1. Install the [Lua](https://marketplace.visualstudio.com/items?itemName=sumneko.lua) extension

2. Create `.luarc.json` in your project root:

```json
{
  "runtime.version": "Lua 5.1",
  "diagnostics.globals": ["object"],
  "workspace.library": ["./annotations"],
  "completion.enable": true
}
```

3. Generate type annotations:

```bash
# This creates annotations/k8s.lua with all Kubernetes types
make generate-stubs
```

4. Use type annotations in your scripts:

```lua
---@type corev1.Pod
local pod = object

-- IDE now provides autocompletion for pod.spec.containers
for i = 1, #pod.spec.containers do
    local container = pod.spec.containers[i]
    -- Autocompletion for container.name, container.image, etc.
end
```

### NeoVim with lua-language-server

1. Install [lua-language-server](https://github.com/luals/lua-language-server)

2. Add to your NeoVim config:

```lua
require('lspconfig').lua_ls.setup({
  settings = {
    Lua = {
      runtime = {
        version = 'Lua 5.1',
      },
      diagnostics = {
        globals = {'object'},
      },
      workspace = {
        library = {
          '/path/to/glua-webhook/annotations',
        },
      },
    },
  },
})
```

3. Generate stubs:

```bash
make generate-stubs
```

## Generating Stubs

### Automatic Generation

Stubs are automatically generated when the webhook starts and registers Kubernetes types.

### Manual Generation

You can generate stubs manually for testing:

```go
package main

import (
    "fmt"
    "github.com/thomas-maurice/glua/pkg/glua"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
    registry := glua.NewTypeRegistry()

    // Register Kubernetes types
    pod := &corev1.Pod{}
    registry.Register(pod)

    deployment := &appsv1.Deployment{}
    registry.Register(deployment)

    // Process all types
    registry.ProcessQueue()

    // Generate stubs
    stubs, err := registry.GenerateStubs()
    if err != nil {
        panic(err)
    }

    fmt.Println(stubs)
}
```

## Type-Safe Script Writing

With type stubs, you can write type-safe Lua scripts:

```lua
---@type corev1.Pod
local pod = object

-- Type checking ensures you don't access non-existent fields
if pod.spec and pod.spec.containers then
    for i = 1, #pod.spec.containers do
        ---@type corev1.Container
        local container = pod.spec.containers[i]

        -- IDE knows container.name is a string
        if container.name == "nginx" then
            -- IDE knows container.image is a string
            container.image = "nginx:1.21"
        end
    end
end
```

## Benefits

### 1. Autocompletion

IDE suggests available fields:

```lua
pod.spec.  -- IDE shows: containers, volumes, nodeSelector, etc.
```

### 2. Type Checking

IDE warns about type mismatches:

```lua
-- Warning: Expected string, got number
pod.metadata.name = 123
```

### 3. Documentation

Hover over fields to see their types and documentation.

### 4. Refactoring

Rename fields safely across all scripts.

## Advanced Usage

### Custom Type Annotations

You can add custom type annotations for your own helpers:

```lua
---@class MyHelper
---@field name string
---@field value number
local MyHelper = {}

---@param name string
---@param value number
---@return MyHelper
function MyHelper:new(name, value)
    return {name = name, value = value}
end

---@type MyHelper
local helper = MyHelper:new("test", 42)
```

### Type Guards

Use type guards for conditional types:

```lua
---@type any
local obj = object

-- Type guard
if obj.kind == "Pod" then
    ---@cast obj corev1.Pod
    -- IDE knows obj is a Pod here
    print(obj.spec.containers[1].name)
elseif obj.kind == "Deployment" then
    ---@cast obj appsv1.Deployment
    -- IDE knows obj is a Deployment here
    print(obj.spec.replicas)
end
```

### Nil Checking with Types

```lua
---@type corev1.Pod
local pod = object

-- Check for nil before accessing
if pod.metadata and pod.metadata.labels then
    ---@type table<string, string>
    local labels = pod.metadata.labels

    for key, value in pairs(labels) do
        -- key and value are both strings
        print(key .. " = " .. value)
    end
end
```

## Troubleshooting

### IDE Not Showing Completions

1. **Check annotations are generated**:
   ```bash
   ls annotations/k8s.lua
   ```

2. **Verify workspace settings**:
   - VSCode: Check `.vscode/settings.json`
   - NeoVim: Check LSP configuration

3. **Reload IDE**:
   - VSCode: Reload window (Cmd+Shift+P → "Reload Window")
   - NeoVim: Restart NeoVim

### Type Mismatches

If IDE shows type errors but script works:

1. **Check Lua version**:
   - glua uses Lua 5.1
   - Some type annotations require Lua 5.3+

2. **Update annotations**:
   ```bash
   make generate-stubs
   ```

3. **Cast types explicitly**:
   ```lua
   ---@cast variable TypeName
   ```

### Missing Types

If a Kubernetes type is not recognized:

1. **Check if type is registered**:
   - Look in `annotations/k8s.lua`
   - Search for the type name

2. **Register manually**:
   - Add type registration in webhook startup
   - Regenerate stubs

## See Also

- [Writing Lua Scripts](writing-scripts.md)
- [glua TypeRegistry](https://github.com/thomas-maurice/glua#type-registry)
- [Lua Language Server](https://github.com/luals/lua-language-server)
