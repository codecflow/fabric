# Fabric

A distributed container orchestration platform for cross-cloud workload scheduling with cost-aware placement and WireGuard mesh networking.

## Architecture

Fabric consists of three main components:

### Weaver (Control Plane)
- **Purpose**: Central orchestration and scheduling
- **Features**:
  - REST/gRPC API for workload management
  - Cost-aware scheduler with multiple placement strategies
  - Multi-provider support (CoreWeave, RunPod, GCP, K8s, KubeVirt, Nosana, AWS-Mac)
  - PostgreSQL state storage with NATS event streaming
  - CRIU snapshot management with Iroh CID integration
  - OpenMeter usage tracking

### Shuttle (Node Runner)
- **Purpose**: Workload execution on compute nodes
- **Features**:
  - WireGuard mesh networking (Tailscale integration)
  - containerd runtime with support for runc, Firecracker, Kata
  - Prometheus metrics exposure with OpenMeter integration
  - Multi-architecture support (linux/amd64, linux/arm64, darwin/arm64)
  - Declarative sidecar management

### Side-cars (Separate Repositories)
- **ctrl**: Keyboard/mouse/screenshot gRPC services
- **stream**: VNC/WebRTC bridge for remote access

## Quick Start

### Prerequisites
- Go 1.24+
- PostgreSQL (for Weaver)
- NATS (for event streaming)
- containerd (for Shuttle)
- Tailscale (optional, for mesh networking)

### Build

```bash
# Build Weaver (control plane)
go build -o weaver cmd/weaver/main.go

# Build Shuttle (node runner)
go build -o shuttle cmd/shuttle/main.go
```

### Configuration

#### Weaver Configuration (weaver.yaml)
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  
database:
  host: "localhost"
  port: 5432
  name: "fabric"
  user: "fabric"
  password: "password"
  
nats:
  url: "nats://localhost:4222"
  
scheduler:
  strategy: "cost_aware"
  
providers:
  kubernetes:
    enabled: true
    kubeconfig: "~/.kube/config"
```

#### Shuttle Configuration (shuttle.yaml)
```yaml
node:
  id: "node-001"
  name: "worker-node-1"
  region: "us-west-2"
  zone: "us-west-2a"
  
weaver:
  endpoint: "http://localhost:8080"
  
runtime:
  type: "containerd"
  socket: "/run/containerd/containerd.sock"
  namespace: "fabric"
  
tailscale:
  enabled: true
  auth_key: "tskey-auth-..."
  hostname: "fabric-node-001"
  
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
```

### Running

#### Start Weaver
```bash
./weaver --config weaver.yaml
```

#### Start Shuttle
```bash
./shuttle --config shuttle.yaml
```

## API Reference

### Weaver REST API

#### Create Workload
```bash
POST /api/v1/workloads
Content-Type: application/json

{
  "name": "my-app",
  "namespace": "default",
  "image": "nginx:latest",
  "resources": {
    "cpu": "1000m",
    "memory": "1Gi"
  },
  "placement": {
    "provider": "kubernetes",
    "region": "us-west-2"
  }
}
```

#### List Workloads
```bash
GET /api/v1/workloads
```

#### Get Workload Status
```bash
GET /api/v1/workloads/{id}
```

#### Delete Workload
```bash
DELETE /api/v1/workloads/{id}
```

### Shuttle Metrics

Shuttle exposes Prometheus metrics on `/metrics`:

- `shuttle_workloads_total`: Total number of workloads
- `shuttle_uptime_seconds`: Shuttle uptime
- `shuttle_memory_usage_bytes`: Memory usage
- `shuttle_cpu_usage_percent`: CPU usage percentage

## Provider Support

### Kubernetes
- Native Kubernetes API integration
- Pod and PVC management
- Resource quota enforcement
- Multi-cluster support

### Cloud Providers
- **CoreWeave**: GPU-optimized compute
- **RunPod**: Serverless GPU containers
- **GCP**: Google Cloud Platform integration
- **AWS**: EC2 and ECS support (including Mac instances)

### Virtualization
- **KubeVirt**: VM workloads on Kubernetes
- **Nosana**: Decentralized compute network

## Networking

Fabric uses WireGuard mesh networking via Tailscale for secure node-to-node communication:

- Automatic node discovery and registration
- Encrypted traffic between all nodes
- NAT traversal and firewall bypass
- Cross-cloud connectivity

## Monitoring & Observability

### Metrics
- Prometheus metrics from all components
- OpenMeter integration for usage tracking
- Custom metrics for workload lifecycle events

### Logging
- Structured logging with configurable levels
- Centralized log aggregation support
- Request tracing and correlation IDs

### Health Checks
- Component health endpoints
- Automated failover and recovery
- Node health monitoring

## Development

### Project Structure
```
fabric/
├── cmd/
│   ├── weaver/          # Control plane entry point
│   └── shuttle/         # Node runner entry point
├── internal/
│   ├── api/             # REST API handlers
│   ├── config/          # Configuration management
│   ├── scheduler/       # Workload scheduling logic
│   ├── providers/       # Cloud provider integrations
│   ├── state/           # State management
│   ├── stream/          # Event streaming
│   ├── metering/        # Usage tracking
│   ├── storage/         # Storage abstractions
│   ├── network/         # Networking utilities
│   ├── types/           # Common data types
│   ├── repository/      # Data persistence
│   └── shuttle/         # Node runner components
│       ├── config/      # Shuttle configuration
│       ├── containerd/  # Container runtime
│       ├── tailscale/   # Mesh networking
│       ├── grpc/        # Weaver communication
│       └── metrics/     # Metrics collection
└── docs/                # Documentation
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Roadmap

- [ ] gRPC API implementation
- [ ] Advanced scheduling algorithms
- [ ] Multi-tenancy support
- [ ] Workload migration capabilities
- [ ] Enhanced security features
- [ ] Performance optimizations
- [ ] Additional provider integrations
