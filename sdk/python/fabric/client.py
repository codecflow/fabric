"""Fabric API client."""
import os
from typing import Any, Dict, List, Optional
import requests


class FabricClient:
    """Client for interacting with the Fabric API."""

    def __init__(
        self,
        gateway_url: Optional[str] = None,
        worker_url: Optional[str] = None,
        timeout: int = 30
    ):
        """Initialize the Fabric client.
        
        Args:
            gateway_url: URL of the gateway server (defaults to FABRIC_GATEWAY_URL env var or http://localhost:8080)
            worker_url: URL of the worker server (defaults to FABRIC_WORKER_URL env var or http://localhost:9090)
            timeout: Request timeout in seconds
        """
        self.gateway_url = gateway_url or os.getenv("FABRIC_GATEWAY_URL", "http://localhost:8080")
        self.worker_url = worker_url or os.getenv("FABRIC_WORKER_URL", "http://localhost:9090")
        self.timeout = timeout

    def create_container(
        self,
        image: str,
        command: List[str],
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        gpu: int = 0,
        persistent: bool = False,
        ports: Optional[List[int]] = None,
    ) -> Dict[str, Any]:
        """Create a container via the worker API.
        
        Args:
            image: Container image name
            command: Command to run
            env: Environment variables
            workdir: Working directory
            gpu: Number of GPUs
            persistent: Whether container should keep running
            ports: List of container ports to expose
            
        Returns:
            Response dict with id, container_id, status, url, and ports
        """
        payload = {
            "image": image,
            "command": command,
            "persistent": persistent,
        }
        
        if env:
            payload["env"] = env
        if workdir:
            payload["workdir"] = workdir
        if gpu > 0:
            payload["gpu"] = gpu
        if ports:
            payload["ports"] = ports

        response = requests.post(
            f"{self.worker_url}/containers",
            json=payload,
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def get_container(self, container_id: str) -> Dict[str, Any]:
        """Get container status.
        
        Args:
            container_id: Container ID
            
        Returns:
            Container status dict
        """
        response = requests.get(
            f"{self.worker_url}/containers/{container_id}",
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def stop_container(self, container_id: str) -> Dict[str, Any]:
        """Stop a running container.
        
        Args:
            container_id: Container ID
            
        Returns:
            Response dict
        """
        response = requests.delete(
            f"{self.worker_url}/containers/{container_id}",
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def list_containers(self) -> Dict[str, Any]:
        """List all containers.
        
        Returns:
            Response dict with placements list
        """
        response = requests.get(
            f"{self.worker_url}/containers",
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()
