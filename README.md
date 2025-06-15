# Fabric - Distributed Compute Platform

Fabric is a distributed compute platform that provides cost-aware scheduling across multiple cloud providers with a focus on performance, reliability, and cost optimization.

## Architecture

### Weaver (Control Plane)
The central control plane that manages workload scheduling and provider coordination.

**Key Components:**
- **REST/gRPC API**: HTTP endpoints for workload management
- **Scheduler**: Cost-aware placement engine with multiple strategies
- **Provider Drivers**: Unified interface for K8s, RunPod, CoreWeave, GCP, AWS, etc.
- **State Management**: Postgres integration for persistence
- **Event Streaming**: NATS/JetStream for real-time updates
- **Metering**: OpenMeter integration for usage tracking

### Shuttle (Node Runner)
Lightweight node agent that runs workloads and manages sidecars.

**Features:**
- WireGuard mesh networking (Tailscale integration)
- Multiple runtime support (runc, Firecracker, Kata)
- Built for linux/amd64, linux/arm64, darwin/arm64
- Sidecar management for ctrl and stream services

### Sidecars
- **ctrl**: Keyboard/mouse/screenshot gRPC services
- **stream**: VNC/WebRTC bridge for remote access

## Current Implementation

### ✅ Completed
- **Type System**: Complete workload, namespace, secret, and provider types
- **Provider Interface**: Unified abstraction for all cloud providers
- **Scheduler**: Cost-aware scheduling with multiple strategies
- **API Foundation**: REST endpoints for workload and provider management
- **Kubernetes Provider**: Basic K8s integration
- **Configuration**: Environment-based configuration system
- **State Management**: Dependency injection container
- **Health Checks**: Built-in monitoring and status endpoints

### 🚧 In Progress
- Concrete provider implementations (RunPod, CoreWeave, AWS, GCP)
- Database integration (Postgres)
- Event streaming (NATS/JetStream)
- Usage metering (OpenMeter)
- P2P storage (Iroh/IPFS)

### 📋 Planned
- Shuttle node runner implementation
- Sidecar services (ctrl, stream)
- CRIU snapshot integration
- Advanced scheduling algorithms
- Multi-tenancy enforcement
- Security and authentication

## API Endpoints

### Health & Status
- `GET /health` - Service health check
- `GET /v1/scheduler/status` - Scheduler status and provider count

### Workload Management
- `POST /v1/workloads` - Create workload
- `GET /v1/workloads/:id` - Get workload details
- `DELETE /v1/workloads/:id` - Delete workload
- `GET /v1/workloads` - List workloads

### Provider Information
- `GET /v1/providers` - List available providers
- `GET /v1/providers/:name/regions` - Get provider regions
- `GET /v1/providers/:name/machine-types` - Get machine types

### Scheduling
- `POST /v1/scheduler/schedule` - Schedule workload
- `GET /v1/scheduler/recommendations` - Get scheduling recommendations
- `GET /v1/scheduler/stats` - Get scheduler statistics

## Configuration

Configuration is managed through environment variables:

```bash
# Server
SERVER_ADDRESS=:8080
SERVER_PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=fabric
DB_USER=fabric
DB_PASSWORD=secret

# NATS
NATS_URL=nats://localhost:4222
NATS_SUBJECT=fabric.events

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Running

```bash
# Build
go build -o weaver cmd/weaver/main.go

# Run
./weaver
```

## Key Features

### Cost-Aware Scheduling
- Multi-provider cost comparison
- Real-time pricing updates
- Cost optimization strategies
- Budget constraints and limits

### Multi-Cloud Support
- Kubernetes clusters
- RunPod GPU instances
- CoreWeave compute
- AWS (including Mac instances)
- Google Cloud Platform
- Azure
- Nosana network

### Event-Driven Architecture
- Real-time workload updates
- Provider status changes
- Cost optimization events
- Usage tracking events

### Mesh Networking
- Secure node-to-node communication
- Tailscale/WireGuard integration
- Cross-cloud connectivity
- Private network isolation

### P2P Storage
- Content-addressed storage
- CRIU snapshot distribution
- Iroh/IPFS integration
- Efficient data transfer

## Development

The codebase follows clean architecture principles with dependency injection:

- `internal/types/` - Core domain types
- `internal/api/` - HTTP API handlers
- `internal/scheduler/` - Scheduling logic
- `internal/providers/` - Cloud provider implementations
- `internal/state/` - Application state management
- `cmd/weaver/` - Main application entry point

All components depend on interfaces, making the system highly testable and modular.
