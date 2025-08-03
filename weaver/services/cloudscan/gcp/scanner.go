package gcp

import (
	"context"
	"time"

	"google.golang.org/api/compute/v1" // todo: use latest sdk.

	"github.com/codecflow/fabric/pkg/dto"
)

type GCPScanner struct {
	svc       *compute.Service
	projectID string
}

// NewGCPScanner constructs a GCPScanner for the given project.
func NewGCPScanner(svc *compute.Service, projectID string) *GCPScanner {
	return &GCPScanner{svc: svc, projectID: projectID}
}

// Scan implements cloudscan.ProviderScanner.
func (s *GCPScanner) Scan(ctx context.Context) ([]dto.Machine, error) {
	var machines []dto.Machine

	// Example: list machineTypes in each zone
	zones := []string{"us-central1-a", "us-central1-b"} // or fetch dynamically
	for _, zone := range zones {
		resp, err := s.svc.MachineTypes.List(s.projectID, zone).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		for _, mt := range resp.Items {
			machines = append(machines, dto.Machine{
				ID:        mt.Name,
				Name:      mt.Name,
				CPU:       int(mt.GuestCpus),
				MemoryMB:  int(mt.MemoryMb),
				Region:    zone,
				Price:     estimatePrice(mt), // you’d implement price lookup
				Timestamp: time.Now(),
			})
		}
	}
	return machines, nil
}

func estimatePrice(mt *compute.MachineType) float64 {
	// TODO: lookup or calculate based on mt.Name
	return 0.0
}
