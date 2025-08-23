"""Fabric Pod abstraction."""
from typing import Any, Dict, List, Optional, Union
from dataclasses import dataclass

from .client import FabricClient
from .image import Image


@dataclass
class PodResult:
    """Result from creating a Pod."""
    
    id: str
    container_id: str
    status: str
    url: Optional[str] = None
    ports: Optional[Dict[int, int]] = None
    
    def __repr__(self) -> str:
        """String representation."""
        parts = [f"PodResult(id={self.id!r}, container_id={self.container_id!r}, status={self.status!r}"]
        if self.url:
            parts.append(f", url={self.url!r}")
        if self.ports:
            parts.append(f", ports={self.ports!r}")
        parts.append(")")
        return "".join(parts)


class Pod:
    """Long-running container with port exposure.
    
    Pods are persistent containers that expose ports and return connection info.
    They're ideal for services, APIs, and interactive workloads.
    
    Example:
        >>> from fabric import Pod, Image
        >>> pod = Pod(
        ...     name="web-server",
        ...     cpu=2,
        ...     memory="512Mi",
        ...     image=Image(base_image="python:3.9-slim"),
        ...     ports=[8000],
        ... )
        >>> result = pod.create(["python", "-m", "http.server", "8000"])
        >>> print(result.url)
        http://localhost:32456
    """

    def __init__(
        self,
        name: str,
        image: Union[Image, str],
        ports: Optional[List[int]] = None,
        cpu: Optional[int] = None,
        memory: Optional[str] = None,
        gpu: int = 0,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        client: Optional[FabricClient] = None,
    ):
        """Initialize a Pod specification.
        
        Args:
            name: Pod name
            image: Image specification (Image object or string)
            ports: List of container ports to expose
            cpu: Number of CPU cores (for future resource scheduling)
            memory: Memory limit (e.g., "512Mi", "2Gi")
            gpu: Number of GPUs to allocate
            env: Environment variables
            workdir: Working directory inside container
            client: Fabric client (uses default if not provided)
        """
        self.name = name
        self.image = image
        self.ports = ports or []
        self.cpu = cpu
        self.memory = memory
        self.gpu = gpu
        self.env = env or {}
        self.workdir = workdir
        self.client = client or FabricClient()
        
        # State
        self._result: Optional[PodResult] = None

    def create(self, command: List[str]) -> PodResult:
        """Create and start the Pod.
        
        Args:
            command: Command to execute in the container
            
        Returns:
            PodResult with connection info
            
        Raises:
            requests.HTTPError: If container creation fails
        """
        # Get image name
        if isinstance(self.image, Image):
            image_name = self.image.to_image_name()
        else:
            image_name = str(self.image)

        # Create container via API
        response = self.client.create_container(
            image=image_name,
            command=command,
            env=self.env,
            workdir=self.workdir,
            gpu=self.gpu,
            persistent=True,  # Pods are always persistent
            ports=self.ports,
        )

        # Store result
        self._result = PodResult(
            id=response["id"],
            container_id=response.get("container_id", ""),
            status=response["status"],
            url=response.get("url"),
            ports=response.get("ports", {}),
        )

        return self._result

    def stop(self) -> None:
        """Stop the running Pod.
        
        Raises:
            ValueError: If Pod hasn't been created yet
            requests.HTTPError: If stop fails
        """
        if not self._result:
            raise ValueError("Pod has not been created yet")
        
        self.client.stop_container(self._result.id)
        self._result.status = "stopped"

    def status(self) -> Dict[str, Any]:
        """Get current Pod status.
        
        Returns:
            Status dict from API
            
        Raises:
            ValueError: If Pod hasn't been created yet
            requests.HTTPError: If status check fails
        """
        if not self._result:
            raise ValueError("Pod has not been created yet")
        
        return self.client.get_container(self._result.id)

    @property
    def result(self) -> Optional[PodResult]:
        """Get the creation result.
        
        Returns:
            PodResult if created, None otherwise
        """
        return self._result

    def __repr__(self) -> str:
        """String representation."""
        image_str = self.image if isinstance(self.image, str) else self.image.base_image
        return f"Pod(name={self.name!r}, image={image_str!r}, ports={self.ports!r})"
