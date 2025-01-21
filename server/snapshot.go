package server

import (
	"captain/k8s"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// VolumeSnapshot resource definition
var volumeSnapshotGVR = schema.GroupVersionResource{
	Group:    "snapshot.storage.k8s.io",
	Version:  "v1",
	Resource: "volumesnapshots",
}

type SnapshotRequest struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
}

type SnapshotResponse struct {
	Success     bool      `json:"success"`
	ID          string    `json:"id"`
	SnapshotID  string    `json:"snapshotId,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type SnapshotListResponse struct {
	ID        string     `json:"id"`
	Snapshots []Snapshot `json:"snapshots"`
	Error     string     `json:"error,omitempty"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SnapshotRestoreRequest struct {
	ID         string `json:"id"`
	SnapshotID string `json:"snapshotId"`
}

// getDynamicClient returns a dynamic client for the Kubernetes API
func (s *Server) getDynamicClient() (dynamic.Interface, error) {
	return dynamic.NewForConfig(s.client.GetConfig())
}

// CreateSnapshot creates a snapshot of a machine's data volume
func (s *Server) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	var req SnapshotRequest
	if r.Header.Get("Content-Type") == "application/json" {
		if err := JSON(r, &req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
	} else {
		req.ID = r.URL.Query().Get("id")
		req.Description = r.URL.Query().Get("description")
	}

	if req.ID == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	// Generate a snapshot ID
	snapshotID := fmt.Sprintf("%s-snap-%d", req.ID, time.Now().Unix())

	// Create a PVC snapshot
	snapshotClass := "csi-hostpath-snapclass" // This should be configured based on your cluster

	// Find the PVC name from the pod
	var pvcName string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName = volume.PersistentVolumeClaim.ClaimName
			break
		}
	}

	if pvcName == "" {
		http.Error(w, "No PVC found for this machine", http.StatusInternalServerError)
		return
	}

	// Create the snapshot using the dynamic client
	dynamicClient, err := s.getDynamicClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create dynamic client: %v", err), http.StatusInternalServerError)
		return
	}

	// Create the snapshot object
	snapshot := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshot",
			"metadata": map[string]interface{}{
				"name": snapshotID,
				"labels": map[string]interface{}{
					"machine-id":    req.ID,
					"snapshot-type": "machine-data",
				},
				"annotations": map[string]interface{}{
					"description": req.Description,
					"created-at":  time.Now().Format(time.RFC3339),
				},
			},
			"spec": map[string]interface{}{
				"volumeSnapshotClassName": snapshotClass,
				"source": map[string]interface{}{
					"persistentVolumeClaimName": pvcName,
				},
			},
		},
	}

	// Create the snapshot
	_, err = dynamicClient.Resource(volumeSnapshotGVR).Namespace(s.client.GetNamespace()).Create(ctx, snapshot, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the snapshot creation
	s.logger.WithFields(logrus.Fields{
		"machine_id":  req.ID,
		"snapshot_id": snapshotID,
		"description": req.Description,
	}).Info("Snapshot created")

	// Return success response
	resp := SnapshotResponse{
		Success:     true,
		ID:          req.ID,
		SnapshotID:  snapshotID,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListSnapshots lists all snapshots for a machine
func (s *Server) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Create the dynamic client
	dynamicClient, err := s.getDynamicClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create dynamic client: %v", err), http.StatusInternalServerError)
		return
	}

	// List all snapshots with the machine-id label
	labelSelector := fmt.Sprintf("machine-id=%s", id)
	snapshots, err := dynamicClient.Resource(volumeSnapshotGVR).Namespace(s.client.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list snapshots: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	snapshotList := make([]Snapshot, 0, len(snapshots.Items))
	for _, snap := range snapshots.Items {
		annotations := snap.GetAnnotations()
		createdAt, _ := time.Parse(time.RFC3339, annotations["created-at"])
		snapshotList = append(snapshotList, Snapshot{
			ID:          snap.GetName(),
			Description: annotations["description"],
			CreatedAt:   createdAt,
		})
	}

	// Return response
	resp := SnapshotListResponse{
		ID:        id,
		Snapshots: snapshotList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RestoreSnapshot restores a machine from a snapshot
func (s *Server) RestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	var req SnapshotRestoreRequest
	if r.Header.Get("Content-Type") == "application/json" {
		if err := JSON(r, &req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
	} else {
		req.ID = r.URL.Query().Get("id")
		req.SnapshotID = r.URL.Query().Get("snapshotId")
	}

	if req.ID == "" || req.SnapshotID == "" {
		http.Error(w, "Missing machine ID or snapshot ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Find the machine
	pod, err := s.client.Find(ctx, req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	// Create the dynamic client
	dynamicClient, err := s.getDynamicClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create dynamic client: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the snapshot
	snapshot, err := dynamicClient.Resource(volumeSnapshotGVR).Namespace(s.client.GetNamespace()).Get(ctx, req.SnapshotID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("Snapshot %s not found", req.SnapshotID), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get snapshot: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Verify the snapshot belongs to this machine
	labels := snapshot.GetLabels()
	if labels["machine-id"] != req.ID {
		http.Error(w, "Snapshot does not belong to this machine", http.StatusBadRequest)
		return
	}

	// Find the PVC name from the pod
	var pvcName string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName = volume.PersistentVolumeClaim.ClaimName
			break
		}
	}

	if pvcName == "" {
		http.Error(w, "No PVC found for this machine", http.StatusInternalServerError)
		return
	}

	// Delete the pod to unmount the PVC
	err = s.client.GetClientset().CoreV1().Pods(s.client.GetNamespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete pod for restore: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete the PVC
	err = s.client.GetClientset().CoreV1().PersistentVolumeClaims(s.client.GetNamespace()).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete PVC for restore: %v", err), http.StatusInternalServerError)
		return
	}

	// Get the restore size from the snapshot status
	restoreSize, found, err := unstructured.NestedString(snapshot.Object, "status", "restoreSize")
	if err != nil || !found {
		restoreSize = "4Gi" // Default size if not found
	}

	// Create a new PVC from the snapshot
	storageClass := "standard-rwo"
	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(restoreSize),
				},
			},
			StorageClassName: &storageClass,
			DataSource: &core.TypedLocalObjectReference{
				APIGroup: &volumeSnapshotGVR.Group,
				Kind:     "VolumeSnapshot",
				Name:     req.SnapshotID,
			},
		},
	}

	// Create the PVC
	_, err = s.client.GetClientset().CoreV1().PersistentVolumeClaims(s.client.GetNamespace()).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create PVC from snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	// Recreate the pod
	_, err = s.client.Create(ctx, k8s.Machine{
		ID:    req.ID,
		Image: pod.Spec.Containers[0].Image,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to recreate pod: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the snapshot restoration
	s.logger.WithFields(logrus.Fields{
		"machine_id":  req.ID,
		"snapshot_id": req.SnapshotID,
	}).Info("Snapshot restored")

	// Return success response
	resp := SnapshotResponse{
		Success:    true,
		ID:         req.ID,
		SnapshotID: req.SnapshotID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DeleteSnapshot deletes a snapshot
func (s *Server) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodDelete) {
		return
	}

	id := r.URL.Query().Get("id")
	snapshotID := r.URL.Query().Get("snapshotId")

	if id == "" || snapshotID == "" {
		http.Error(w, "Missing machine ID or snapshot ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Create the dynamic client
	dynamicClient, err := s.getDynamicClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create dynamic client: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the snapshot
	snapshot, err := dynamicClient.Resource(volumeSnapshotGVR).Namespace(s.client.GetNamespace()).Get(ctx, snapshotID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("Snapshot %s not found", snapshotID), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get snapshot: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Verify the snapshot belongs to this machine
	labels := snapshot.GetLabels()
	if labels["machine-id"] != id {
		http.Error(w, "Snapshot does not belong to this machine", http.StatusBadRequest)
		return
	}

	// Delete the snapshot
	err = dynamicClient.Resource(volumeSnapshotGVR).Namespace(s.client.GetNamespace()).Delete(ctx, snapshotID, metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the snapshot deletion
	s.logger.WithFields(logrus.Fields{
		"machine_id":  id,
		"snapshot_id": snapshotID,
	}).Info("Snapshot deleted")

	// Return success response
	resp := SnapshotResponse{
		Success:    true,
		ID:         id,
		SnapshotID: snapshotID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
