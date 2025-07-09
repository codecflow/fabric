package handlers

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/codecflow/fabric/weaver/internal/state"
	"github.com/codecflow/fabric/weaver/weaver/proto/weaver"
)

type ProviderHandler struct {
	appState *state.State
	logger   *logrus.Logger
}

func NewProviderHandler(appState *state.State, logger *logrus.Logger) *ProviderHandler {
	return &ProviderHandler{
		appState: appState,
		logger:   logger,
	}
}

func (h *ProviderHandler) List(ctx context.Context, req *emptypb.Empty) (*weaver.ListProvidersResponse, error) {
	providers := make([]string, 0, len(h.appState.Providers))
	for name := range h.appState.Providers {
		providers = append(providers, name)
	}
	return &weaver.ListProvidersResponse{Providers: providers}, nil
}

func (h *ProviderHandler) GetRegions(ctx context.Context, req *weaver.GetProviderRegionsRequest) (*weaver.GetProviderRegionsResponse, error) {
	provider, exists := h.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	resources, err := provider.GetAvailableResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider resources: %v", err)
	}

	var regions []string
	for _, region := range resources.Regions {
		if region.Available {
			regions = append(regions, region.Name)
		}
	}

	return &weaver.GetProviderRegionsResponse{
		Regions: regions,
	}, nil
}

func (h *ProviderHandler) GetMachineTypes(ctx context.Context, req *weaver.GetProviderMachineTypesRequest) (*weaver.GetProviderMachineTypesResponse, error) {
	provider, exists := h.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	pricing, err := provider.GetPricing(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider pricing: %v", err)
	}

	resources, err := provider.GetAvailableResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider resources: %v", err)
	}

	var machineTypes []*weaver.MachineType

	// Standard CPU/Memory machine types
	standardTypes := []struct {
		name   string
		cpu    string
		memory string
		cpuNum float64
		memGB  float64
	}{
		{"micro", "0.5", "1Gi", 0.5, 1.0},
		{"small", "1", "2Gi", 1.0, 2.0},
		{"medium", "2", "4Gi", 2.0, 4.0},
		{"large", "4", "8Gi", 4.0, 8.0},
		{"xlarge", "8", "16Gi", 8.0, 16.0},
		{"2xlarge", "16", "32Gi", 16.0, 32.0},
		{"4xlarge", "32", "64Gi", 32.0, 64.0},
		{"8xlarge", "64", "128Gi", 64.0, 128.0},
		{"16xlarge", "128", "256Gi", 128.0, 256.0},
	}

	for _, mt := range standardTypes {
		price := mt.cpuNum*pricing.CPU.Amount + mt.memGB*pricing.Memory.Amount
		machineTypes = append(machineTypes, &weaver.MachineType{
			Name:         mt.name,
			Cpu:          mt.cpu,
			Memory:       mt.memory,
			Gpu:          "",
			PricePerHour: price,
		})
	}

	// GPU machine types
	for gpuType, gpuInfo := range resources.GPU.Types {
		if gpuInfo.Available > 0 {
			machineTypes = append(machineTypes, &weaver.MachineType{
				Name:         fmt.Sprintf("gpu-%s", gpuType),
				Cpu:          "8",
				Memory:       "32Gi",
				Gpu:          gpuType,
				PricePerHour: gpuInfo.PricePerHour,
			})
		}
	}

	return &weaver.GetProviderMachineTypesResponse{
		MachineTypes: machineTypes,
	}, nil
}
