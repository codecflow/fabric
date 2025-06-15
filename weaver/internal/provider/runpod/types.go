package runpod

// Pod represents a RunPod instance
type Pod struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ImageName     string            `json:"imageName"`
	GPUType       string            `json:"gpuTypeId"`
	GPUCount      int               `json:"gpuCount"`
	VCPUCount     int               `json:"vcpuCount"`
	MemoryInGB    int               `json:"memoryInGb"`
	ContainerDisk int               `json:"containerDiskInGb"`
	VolumeDisk    int               `json:"volumeDiskInGb"`
	VolumeMount   string            `json:"volumeMountPath"`
	Ports         string            `json:"ports"`
	Env           map[string]string `json:"env"`
	MachineID     string            `json:"machineId"`
	Status        string            `json:"desiredStatus"`
	Runtime       Runtime           `json:"runtime"`
}

// Runtime represents runtime information
type Runtime struct {
	UptimeInSeconds int    `json:"uptimeInSeconds"`
	Ports           []Port `json:"ports"`
	GPUs            []GPU  `json:"gpus"`
}

// Port represents a port mapping
type Port struct {
	IP          string `json:"ip"`
	IsIpPublic  bool   `json:"isIpPublic"`
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort"`
	Type        string `json:"type"`
}

// GPU represents GPU information
type GPU struct {
	ID            string `json:"id"`
	GPUUtilPct    int    `json:"gpuUtilPct"`
	MemoryUtilPct int    `json:"memoryUtilPct"`
}

// GPUType represents available GPU types
type GPUType struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	MemoryInGB     int    `json:"memoryInGb"`
	SecureCloud    bool   `json:"secureCloud"`
	CommunityCloud bool   `json:"communityCloud"`
	LowestPrice    Price  `json:"lowestPrice"`
}

// Price represents pricing information
type Price struct {
	MinimumBidPrice      float64 `json:"minimumBidPrice"`
	UninterruptiblePrice float64 `json:"uninterruptiblePrice"`
}
