package dto

import (
	"time"
)

type CloudProvider string

const (
	AWS   CloudProvider = "AWS"
	GCP   CloudProvider = "GCP"
	Azure CloudProvider = "Azure"
)

type (
	Machine struct {
		ID        string
		Name      string
		Provider  CloudProvider
		CPU       int // vCPU count
		MemoryMB  int
		Region    string
		Price     float64
		Timestamp time.Time
	}
)
