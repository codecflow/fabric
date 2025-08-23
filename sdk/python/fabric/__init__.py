"""Fabric Python SDK.

A serverless GPU orchestration platform for AI/ML workloads.
"""

from .client import FabricClient
from .image import Image
from .pod import Pod, PodResult

__version__ = "0.1.0"

__all__ = [
    "FabricClient",
    "Image",
    "Pod",
    "PodResult",
]
