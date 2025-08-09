package container

// Spec defines the container specification
type Spec struct {
	// Image is the container image to run (e.g., "python:3.11", "alpine:latest")
	Image string

	// Command is the command to execute in the container
	Command []string

	// Env is a map of environment variables to set in the container
	Env map[string]string

	// WorkDir is the working directory inside the container
	WorkDir string
}

// Result contains the output from a container execution
type Result struct {
	// ExitCode is the exit code returned by the container
	ExitCode int

	// Stdout is the standard output from the container
	Stdout string

	// Stderr is the standard error output from the container
	Stderr string
}
