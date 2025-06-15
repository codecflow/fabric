# Fabric - Multi-Cloud Workload Orchestration

Fabric is a multi-cloud workload orchestration system composed of:
- **Weaver**: Control plane with REST/gRPC APIs, scheduler, and cost-aware placement
- **Shuttle**: Node runner that joins WireGuard mesh and manages workloads via containerd
- **Sidecars**: ctrl (input/output) and stream (VNC/WebRTC bridge) components

## Architecture

```mermaid
graph TB
    subgraph "Fabric System"
        subgraph "Control Plane"
            W[Weaver<br/>REST/gRPC API<br/>Scheduler<br/>Cost-aware Placement]
            DB[(PostgreSQL<br/>State Storage)]
            NATS[NATS<br/>Event Bus]
        end
        
        subgraph "Node Runners"
            S1[Shuttle 1<br/>WireGuard Mesh<br/>containerd]
            S2[Shuttle 2<br/>WireGuard Mesh<br/>containerd]
            S3[Shuttle N<br/>WireGuard Mesh<br/>containerd]
        end
        
        subgraph "Side-cars"
            CTRL[ctrl<br/>Input/Output<br/>gRPC]
            STREAM[stream<br/>VNC/WebRTC<br/>Bridge]
        end
        
        subgraph "Cloud Providers"
            K8S[Kubernetes<br/>Clusters]
            RP[RunPod<br/>GPU Cloud]
            CW[CoreWeave<br/>GPU Cloud]
            AWS[AWS<br/>EC2/EKS]
            GCP[GCP<br/>GCE/GKE]
        end
    end
    
    W --> DB
    W --> NATS
    W --> K8S
    W --> RP
    W --> CW
    W --> AWS
    W --> GCP
    
    NATS --> S1
    NATS --> S2
    NATS --> S3
    
    S1 --> CTRL
    S1 --> STREAM
    S2 --> CTRL
    S2 --> STREAM
    S3 --> CTRL
    S3 --> STREAM
```

### Component Overview
- **Control Plane (Weaver)**: REST/gRPC API, scheduler, cost-aware placement, provider drivers
- **Node Runner (Shuttle)**: Joins WireGuard mesh, manages workloads via containerd
- **Side-cars**: ctrl (input/output), stream (VNC/WebRTC bridge)

## Features

- Multi-cloud workload scheduling (K8s, RunPod, CoreWeave, AWS, GCP)
- Cost-aware placement optimization
- Provider abstraction layer
- REST API for workload management
- Configuration via CLI flags, config files, or environment variables

## Quick Start

### Build

```bash
# Build all components
make build

# Or build weaver specifically
make weaver

# Clean build artifacts
make clean
```

### Run

```bash
# With default settings (K8s provider enabled)
./build/weaver --k-8-s-enabled

# With custom config
./build/weaver --config config.yaml

# See all options
./build/weaver --help
```

### Configuration

Configuration can be provided via:
1. Command line flags
2. YAML config file
3. Environment variables

Example config.yaml:
```yaml
server:
  address: ":8080"

database:
  host: "localhost"
  port: 5432
  user: "fabric"
  password: "fabric"
  database: "fabric"

nats:
  url: "nats://localhost:4222"

providers:
  k8s:
    enabled: true
    namespace: "default"
  runpod:
    enabled: false
    api_key: ""
  coreweave:
    enabled: false
    api_key: ""
```

## API Endpoints

### Health Check
- `GET /health` - Service health status

### Workload Management
- `POST /v1/workloads` - Create workload
- `GET /v1/workloads/:id` - Get workload status
- `DELETE /v1/workloads/:id` - Delete workload
- `GET /v1/workloads` - List workloads

### Provider Information
- `GET /v1/providers` - List available providers
- `GET /v1/providers/:name/regions` - List provider regions
- `GET /v1/providers/:name/machine-types` - List provider machine types

### Scheduler
- `GET /v1/scheduler/status` - Scheduler status

## Providers

### Kubernetes (K8s)
- Local or remote Kubernetes clusters
- Pod-based workload execution
- Resource quotas and limits

### RunPod
- GPU-focused cloud provider
- RTX 4090, RTX 4080, A100, H100 instances
- API-based provisioning

### CoreWeave
- High-performance GPU cloud
- A6000, A100, H100 instances
- Multiple regions (Chicago, New York, Las Vegas)

## Development

### Project Structure
```
cmd/                 # Build targets → binaries
  weaver/            # Controller / scheduler
  shuttle/           # Node runner
internal/            # Private libraries
  api/               # REST + gRPC handlers (weaver)
  scheduler/         # Bin-packer, cost logic
  provider/          # Pluggable drivers
    k8s/             # Kubernetes provider
    runpod/          # RunPod provider
    coreweave/       # CoreWeave provider
  mesh/              # Tailscale / WG helpers
  snapshot/          # CRIU + Iroh helpers
  db/                # Postgres migrations + sqlc code
  config/            # Configuration management
  state/             # Application state management
  util/              # Common utilities
images/              # Docker images
  weaver/            # Weaver container
  shuttle/           # Shuttle container
```

### Adding New Providers

1. Create new provider package in `internal/provider/`
2. Implement the `Provider` interface
3. Add configuration struct to `internal/config/`
4. Register provider in `internal/state/state.go`

## Dependencies

- PostgreSQL (for state storage)
- NATS (for event publishing)
- Kubernetes cluster (if using K8s provider)
- Provider API keys (for cloud providers)

## Docker

Build and run with Docker:

```bash
docker build -f images/Dockerfile.weaver -t weaver .
docker run -p 8080:8080 weaver
