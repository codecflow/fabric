# Captain 

> [!WARNING]  
> This project is still in progress. Updates are coming soon and APIs may change.

Core infrastructure component for CodecFlow - a platform providing on-demand cloud desktops for AI agents.

## What is Captain?

Captain is a Kubernetes-based service that provisions and manages virtual machines for AI agents. It enables:

- Secure machine provisioning in Trusted Execution Environments (TEE)
- Model Context Protocol (MCP) integration for AI agent communication
- Command execution and file transfers
- Resource monitoring and snapshot management

## Architecture

Built on Kubernetes for scalability and reliability, Captain provides isolated environments for AI agents to perform OS-level tasks securely.

```mermaid
graph TD
    A[AI Agent] -->|API Request| B[Captain Server]
    B -->|Create Machine| C[Kubernetes API]
    C -->|Create Pod| D[Machine Pod]
    C -->|Create PVC| E[Persistent Storage]
    D -->|Mount| E
    A -->|Connect| D
    B -->|Execute Commands| D
    B -->|Get Logs| D
    B -->|Upload/Download Files| D
    B -->|Monitor Resources| D
    B -->|Create/Restore Snapshots| E
    
    subgraph "Kubernetes Cluster"
        C
        D
        E
    end
```

## Request Flow

The following diagram illustrates the flow of a machine creation request:

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant Captain as Captain Server
    participant K8s as Kubernetes API
    participant Pod as Machine Pod
    participant Storage as Persistent Storage

    Agent->>Captain: Create Machine Request
    opt Template-based creation
        Captain->>Captain: Resolve template
    end
    Captain->>K8s: Create PVC
    K8s->>Storage: Allocate storage
    Captain->>K8s: Create Pod
    K8s->>Pod: Schedule and start
    Pod->>Storage: Mount volume
    Captain->>Agent: Return machine details
    
    Agent->>Captain: Connect to machine
    Captain->>Pod: Establish connection
    Captain->>Agent: Return connection stream
    
    Agent->>Captain: Execute command
    Captain->>Pod: Run command
    Pod->>Captain: Return result
    Captain->>Agent: Return command output
```

## TODO

- [ ] Add authentication and authorization
- [x] Implement resource quotas and limits
- [x] Add support for custom machine templates
- [ ] Enhance monitoring with Prometheus integration
- [ ] Improve documentation and API reference
- [ ] Implement automated testing
- [ ] Add support for GPU acceleration
- [ ] Implement network isolation between machines
- [x] Add machine health checks and auto-recovery
- [ ] Support for multiple cloud providers
- [ ] Add Firecracker support for lightweight virtualization
