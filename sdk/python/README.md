# Fabric Python SDK

Python SDK for Fabric - a serverless GPU orchestration platform.

## Installation

```bash
# Install in development mode
cd sdk/python
pip install -e .
```

## Quick Start

### Creating a Pod

```python
from fabric import Pod, Image

# Create a Pod that runs a simple HTTP server
pod = Pod(
    name="web-server",
    cpu=2,
    memory="512Mi",
    image=Image(
        base_image="python:3.9-slim",
        python_packages=["requests"],
    ),
    ports=[8000],
)

# Create the pod container
result = pod.create(["python", "-m", "http.server", "8000"])

print("Container ID:", result.container_id)
print("URL:", result.url)
```

## Configuration

The SDK looks for these environment variables:

- `FABRIC_WORKER_URL`: Worker API URL (default: `http://localhost:9090`)
- `FABRIC_GATEWAY_URL`: Gateway API URL (default: `http://localhost:8080`)

## Requirements

- Python 3.8+
- Running Fabric gateway and worker services

## API Reference

### Pod

Long-running containers with port exposure.

```python
Pod(
    name: str,
    image: Union[Image, str],
    ports: Optional[List[int]] = None,
    cpu: Optional[int] = None,
    memory: Optional[str] = None,
    gpu: int = 0,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
)
```

### Image

Container image specification.

```python
Image(
    base_image: str = "python:3.11-slim",
    python_packages: Optional[List[str]] = None,
    system_packages: Optional[List[str]] = None,
)
```

### FabricClient

Low-level API client.

```python
FabricClient(
    gateway_url: Optional[str] = None,
    worker_url: Optional[str] = None,
    timeout: int = 30,
)
```
