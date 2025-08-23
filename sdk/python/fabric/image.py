"""Fabric Image specification."""
from typing import List, Optional


class Image:
    """Container image specification.
    
    For MVP, this is a simple wrapper around a base image with optional packages.
    In the future, this will support building custom images.
    """

    def __init__(
        self,
        base_image: str = "python:3.11-slim",
        python_packages: Optional[List[str]] = None,
        system_packages: Optional[List[str]] = None,
    ):
        """Initialize an Image specification.
        
        Args:
            base_image: Base Docker image to use
            python_packages: Python packages to install (for future image building)
            system_packages: System packages to install (for future image building)
        """
        self.base_image = base_image
        self.python_packages = python_packages or []
        self.system_packages = system_packages or []

    def add_python_packages(self, packages: List[str]) -> "Image":
        """Add Python packages to the image.
        
        Args:
            packages: List of Python package names
            
        Returns:
            Self for method chaining
        """
        self.python_packages.extend(packages)
        return self

    def add_system_packages(self, packages: List[str]) -> "Image":
        """Add system packages to the image.
        
        Args:
            packages: List of system package names
            
        Returns:
            Self for method chaining
        """
        self.system_packages.extend(packages)
        return self

    def to_image_name(self) -> str:
        """Get the image name to use.
        
        For MVP, just return the base image.
        In the future, this will build a custom image if packages are specified.
        
        Returns:
            Docker image name
        """
        # TODO: Future enhancement - build custom image if packages specified
        if self.python_packages or self.system_packages:
            # For now, just warn that packages will be ignored
            print(f"Warning: Custom package installation not yet implemented. Using base image: {self.base_image}")
        
        return self.base_image
