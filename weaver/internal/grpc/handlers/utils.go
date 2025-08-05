package handlers

import (
	"crypto/rand"
	"encoding/hex"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/weaver/proto/weaver"

	// todo: service should separated from here.
	"github.com/codecflow/fabric/weaver/services/scheduler"
)

// generateID creates a random ID for workloads
func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// convertWorkloadSpec converts protobuf WorkloadSpec to internal WorkloadSpec
func convertWorkloadSpec(spec *weaver.WorkloadSpec) workload.Spec {
	if spec == nil {
		return workload.Spec{}
	}

	result := workload.Spec{
		Image:   spec.Image,
		Command: spec.Command,
		Args:    spec.Args,
		Env:     spec.Env,
	}

	if spec.Resources != nil {
		result.Resources = workload.ResourceRequests{
			CPU:    spec.Resources.Cpu,
			Memory: spec.Resources.Memory,
			GPU:    spec.Resources.Gpu,
		}
	}

	for _, volume := range spec.Volumes {
		result.Volumes = append(result.Volumes, workload.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
			ContentID: volume.ContentId,
		})
	}

	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, workload.Port{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	for _, sidecar := range spec.Sidecars {
		result.Sidecars = append(result.Sidecars, workload.SidecarSpec{
			Name:    sidecar.Name,
			Image:   sidecar.Image,
			Command: sidecar.Command,
			Args:    sidecar.Args,
			Env:     sidecar.Env,
		})
	}

	result.Restart = workload.RestartPolicy(spec.RestartPolicy)

	if spec.Placement != nil {
		result.Placement = workload.PlacementSpec{
			Provider:   spec.Placement.Provider,
			Region:     spec.Placement.Region,
			Zone:       spec.Placement.Zone,
			NodeLabels: spec.Placement.NodeLabels,
		}

		for _, toleration := range spec.Placement.Tolerations {
			result.Placement.Tolerations = append(result.Placement.Tolerations, workload.Toleration{
				Key:      toleration.Key,
				Operator: toleration.Operator,
				Value:    toleration.Value,
				Effect:   toleration.Effect,
			})
		}
	}

	return result
}

// convertWorkloadStatus converts internal WorkloadStatus to protobuf WorkloadStatus
func convertWorkloadStatus(status *workload.Status) *weaver.WorkloadStatus {
	if status == nil {
		return nil
	}

	result := &weaver.WorkloadStatus{
		Phase:        string(status.Phase),
		Message:      status.Message,
		Reason:       status.Reason,
		RestartCount: status.RestartCount,
		NodeId:       status.NodeID,
		Provider:     status.Provider,
		TailscaleIp:  status.TailscaleIP,
		ContainerId:  status.ContainerID,
		SnapshotId:   status.SnapshotID,
	}

	if status.StartTime != nil {
		result.StartTime = timestamppb.New(*status.StartTime)
	}
	if status.FinishTime != nil {
		result.FinishTime = timestamppb.New(*status.FinishTime)
	}
	if status.LastSnapshot != nil {
		result.LastSnapshot = timestamppb.New(*status.LastSnapshot)
	}

	return result
}

// convertWorkloadSpecToProto converts internal WorkloadSpec to protobuf WorkloadSpec
func convertWorkloadSpecToProto(spec *workload.Spec) *weaver.WorkloadSpec {
	if spec == nil {
		return nil
	}

	result := &weaver.WorkloadSpec{
		Image:         spec.Image,
		Command:       spec.Command,
		Args:          spec.Args,
		Env:           spec.Env,
		RestartPolicy: string(spec.Restart),
	}

	result.Resources = &weaver.ResourceRequests{
		Cpu:    spec.Resources.CPU,
		Memory: spec.Resources.Memory,
		Gpu:    spec.Resources.GPU,
	}

	for _, volume := range spec.Volumes {
		result.Volumes = append(result.Volumes, &weaver.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
			ContentId: volume.ContentID,
		})
	}

	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, &weaver.Port{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	for _, sidecar := range spec.Sidecars {
		result.Sidecars = append(result.Sidecars, &weaver.SidecarSpec{
			Name:    sidecar.Name,
			Image:   sidecar.Image,
			Command: sidecar.Command,
			Args:    sidecar.Args,
			Env:     sidecar.Env,
		})
	}

	if spec.Placement.Provider != "" || spec.Placement.Region != "" || spec.Placement.Zone != "" {
		result.Placement = &weaver.PlacementSpec{
			Provider:   spec.Placement.Provider,
			Region:     spec.Placement.Region,
			Zone:       spec.Placement.Zone,
			NodeLabels: spec.Placement.NodeLabels,
		}

		for _, toleration := range spec.Placement.Tolerations {
			result.Placement.Tolerations = append(result.Placement.Tolerations, &weaver.Toleration{
				Key:      toleration.Key,
				Operator: toleration.Operator,
				Value:    toleration.Value,
				Effect:   toleration.Effect,
			})
		}
	}

	return result
}

// convertProviderStats converts scheduler provider stats to protobuf format
func convertProviderStats(providerStats map[string]*scheduler.ProviderStats) map[string]int32 {
	result := make(map[string]int32)
	for provider, stats := range providerStats {
		if stats != nil {
			result[provider] = int32(stats.TotalScheduled) // nolint:gosec
		}
	}
	return result
}

// calculateTotalCost calculates total cost from provider stats
func calculateTotalCost(providerStats map[string]*scheduler.ProviderStats) float64 {
	var totalCost float64
	for _, stats := range providerStats {
		if stats != nil {
			totalCost += stats.AverageCost
		}
	}
	return totalCost
}
