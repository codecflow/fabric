package scheduler

import (
	"database/sql"
	"fabric/internal/provider"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	providers map[string]provider.Provider
	db        *sql.DB
	nats      *nats.Conn
	logger    *logrus.Logger
}

type ScheduleRequest struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	CPU         int               `json:"cpu"`
	Memory      int               `json:"memory"`
	GPU         *provider.GPU     `json:"gpu,omitempty"`
	Region      string            `json:"region,omitempty"`
	Provider    string            `json:"provider,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	MaxCostHour float64           `json:"max_cost_hour,omitempty"`
}

type Placement struct {
	Provider    string                `json:"provider"`
	Region      string                `json:"region"`
	MachineType string                `json:"machine_type"`
	CostPerHour float64               `json:"cost_per_hour"`
	Spec        provider.InstanceSpec `json:"spec"`
}

func New(providers map[string]provider.Provider, db *sql.DB, nats *nats.Conn, logger *logrus.Logger) *Scheduler {
	return &Scheduler{
		providers: providers,
		db:        db,
		nats:      nats,
		logger:    logger,
	}
}

func (s *Scheduler) Schedule(req ScheduleRequest) (*Placement, error) {
	s.logger.Infof("Scheduling request: %+v", req)

	// If provider is specified, use it directly
	if req.Provider != "" {
		if provider, exists := s.providers[req.Provider]; exists {
			return s.scheduleOnProvider(provider, req)
		}
		s.logger.Warnf("Specified provider %s not available", req.Provider)
	}

	// Find best placement across all providers
	var bestPlacement *Placement
	var bestCost float64

	for name, provider := range s.providers {
		placement, err := s.scheduleOnProvider(provider, req)
		if err != nil {
			s.logger.Warnf("Failed to schedule on provider %s: %v", name, err)
			continue
		}

		if bestPlacement == nil || placement.CostPerHour < bestCost {
			bestPlacement = placement
			bestCost = placement.CostPerHour
		}
	}

	if bestPlacement == nil {
		return nil, fmt.Errorf("no suitable placement found")
	}

	s.logger.Infof("Best placement: %+v", bestPlacement)
	return bestPlacement, nil
}

func (s *Scheduler) scheduleOnProvider(p provider.Provider, req ScheduleRequest) (*Placement, error) {
	// Get available machine types
	machineTypes, err := p.ListMachineTypes()
	if err != nil {
		return nil, err
	}

	// Find suitable machine type
	var bestMachine *provider.MachineType
	for _, machine := range machineTypes {
		if machine.CPU >= req.CPU && machine.Memory >= req.Memory {
			// Check GPU requirements
			if req.GPU != nil {
				if machine.GPU == nil || machine.GPU.Type != req.GPU.Type || machine.GPU.Count < req.GPU.Count {
					continue
				}
			}
			bestMachine = &machine
			break
		}
	}

	if bestMachine == nil {
		return nil, fmt.Errorf("no suitable machine type found")
	}

	// Get regions
	regions, err := p.ListRegions()
	if err != nil {
		return nil, err
	}

	// Select region (use specified or first available)
	var selectedRegion string
	if req.Region != "" {
		selectedRegion = req.Region
	} else if len(regions) > 0 {
		selectedRegion = regions[0].ID
	} else {
		return nil, fmt.Errorf("no regions available")
	}

	// Get cost
	cost, err := p.GetCostPerHour(bestMachine.ID, selectedRegion)
	if err != nil {
		return nil, err
	}

	// Check cost limit
	if req.MaxCostHour > 0 && cost > req.MaxCostHour {
		return nil, fmt.Errorf("cost %.2f exceeds limit %.2f", cost, req.MaxCostHour)
	}

	return &Placement{
		Provider:    p.Name(),
		Region:      selectedRegion,
		MachineType: bestMachine.ID,
		CostPerHour: cost,
		Spec: provider.InstanceSpec{
			Provider:    p.Name(),
			Region:      selectedRegion,
			MachineType: bestMachine.ID,
			Image:       req.Image,
			Name:        req.Name,
			Env:         req.Env,
			Labels:      req.Labels,
		},
	}, nil
}
