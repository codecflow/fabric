# Fabric

A distributed workload orchestration system for cross-cloud computing, built from the ground up for modern cloud-native applications.

## Architecture

Fabric consists of three main components:

### Weaver (Control Plane)
The central orchestration service that manages workload scheduling and placement across multiple cloud providers.

**Features:**
- gRPC API for workload management
- Cost-aware scheduling with multiple strategies
- Provider drivers for CoreWeave, RunPod, GCP, K8s, KubeVirt, Nosana, AWS-Mac
- PostgreSQL state storage with NATS event streaming
- CRIU snapshot management with Iroh content addressing
- Built-in proxy for workload access

**Key Components:**
- Scheduler with pluggable strategies (cost-optimized, performance, balanced)
- Provider abstraction layer
- State management with PostgreSQL
- Event streaming with NATS
- Snapshot management with CRIU → Iroh CID
- Usage metering bridge

### Shuttle (Node Runner)
The node agent that runs on compute nodes and manages workload execution.

**Features:**
- WireGuard mesh networking (Tailscale integration)
- containerd integration for container management
- Support for runc, Firecracker, and Kata containers
- OpenMeter integration for usage tracking
- Multi-architecture support (linux/amd64, linux/arm64, darwin/arm64)

**Key Components:**
- Tailscale mesh networking
- containerd runtime management
- Metrics collection and reporting
- gRPC server for control plane communication

### Gauge (Metering)
Standalone metering service for usage tracking and billing.

**Features:**
- OpenMeter integration
- Usage data collection from Shuttle nodes
- Billing and cost tracking
- Export capabilities for external systems

## Side-cars

Fabric supports declarative side-car containers launched by Shuttle:

- **ctrl**: Keyboard/mouse/screenshot gRPC service
- **stream**: VNC/WebRTC bridge for remote access

## Quick Start

### Prerequisites

- Go 1.24+
- Protocol Buffers compiler (`protoc`)
- Docker (optional, for containerized deployment)

### Building

```bash
# Install dependencies
make deps

# Build all components
make build

# Or build individually
make weaver  # Control plane
make shuttle # Node runner
make gauge   # Metering service
```

### Configuration

Copy and modify the configuration file:

```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your settings
```

### Running

#### Development Mode

```bash
# Start Weaver (control plane)
make dev-weaver

# Start Shuttle (node runner)
make dev-shuttle

# Start Gauge (metering)
make dev-gauge
```

#### Production Mode

```bash
# Start services
./bin/weaver
./bin/shuttle
./bin/gauge
```

## API

Fabric exposes a gRPC API for workload management. The API includes:

### Workload Management
- `CreateWorkload` - Create and schedule a new workload
- `GetWorkload` - Retrieve workload details
- `ListWorkloads` - List workloads with filtering
- `DeleteWorkload` - Remove a workload

### Provider Management
- `ListProviders` - Get available cloud providers
- `GetProviderRegions` - List regions for a provider
- `GetProviderMachineTypes` - Get available machine types

### Scheduler
- `GetSchedulerStatus` - Check scheduler health
- `ScheduleWorkload` - Manually schedule a workload
- `GetRecommendations` - Get placement recommendations
- `GetSchedulerStats` - Retrieve scheduling statistics

### Health Check
- `HealthCheck` - Service health status

## Configuration

Fabric uses YAML configuration files. Key sections include:

```yaml
server:
  address: ":8080"

database:
  host: "localhost"
  port: 5432
  name: "fabric"
  user: "fabric"
  password: "password"

nats:
  url: "nats://localhost:4222"

proxy:
  enabled: true
  port: 8081

providers:
  kubernetes:
    enabled: true
    kubeconfig: "~/.kube/config"
  
  coreweave:
    enabled: false
    api_key: ""
    
  runpod:
    enabled: false
    api_key: ""
```

## Scheduling

Fabric supports multiple scheduling strategies:

- **lowest_cost** - Minimize cost per hour
- **best_performance** - Optimize for performance
- **balanced** - Balance cost and performance
- **high_availability** - Prioritize reliability
- **custom** - User-defined weights

Scheduling considers:
- Resource requirements (CPU, memory, GPU)
- Cost constraints
- Provider availability
- Network latency
- Compliance requirements

## Networking

Fabric uses Tailscale for secure mesh networking between nodes:

- Automatic node discovery
- Encrypted communication
- NAT traversal
- Cross-cloud connectivity

## Storage

Workload state and snapshots are managed through:

- **PostgreSQL** - Persistent state storage
- **NATS** - Event streaming and pub/sub
- **Iroh** - Content-addressed snapshot storage
- **CRIU** - Container checkpoint/restore

## Monitoring

Built-in observability features:

- Prometheus metrics export
- Structured logging (JSON)
- Health check endpoints
- Usage tracking with OpenMeter

## Development

### Project Structure

```
fabric/
├── cmd/                    # Main applications
│   ├── weaver/            # Control plane
│   ├── shuttle/           # Node runner
│   └── gauge/             # Metering service
├── internal/              # Internal packages
│   ├── api/               # HTTP API (legacy)
│   ├── grpc/              # gRPC server
│   ├── scheduler/         # Scheduling logic
│   ├── providers/         # Cloud provider drivers
│   ├── state/             # State management
│   ├── types/             # Common types
│   └── ...
├── proto/                 # Protocol buffer definitions
└── bin/                   # Built binaries
```

### Adding Providers

To add a new cloud provider:

1. Implement the `Provider` interface in `internal/providers/`
2. Add provider configuration to config schema
3. Register the provider in the provider factory
4. Add provider-specific machine types and regions

### Testing

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/scheduler/...
```

## Deployment

### Kubernetes

Fabric can be deployed on Kubernetes using the provided manifests:

```bash
kubectl apply -f manifests/
```

### Docker

Build and run with Docker:

```bash
# Build images
docker build -t fabric/weaver .
docker build -t fabric/shuttle .
docker build -t fabric/gauge .

# Run with docker-compose
docker-compose up
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[License information]

## Support

For questions and support:
- GitHub Issues
- Documentation: [link]
- Community: [link]
