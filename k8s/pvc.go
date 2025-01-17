package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func (c *Client) FindPVC(ctx context.Context, name string) (bool, error) {
	_, err := c.client.CoreV1().PersistentVolumeClaims(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check PVC existence: %v", err)
	}
	return true, nil
}

func (c *Client) CreatePVC(ctx context.Context, name, size string) error {
	class := "standard-rwo"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: &class,
		},
	}

	_, err := c.client.CoreV1().PersistentVolumeClaims(c.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC %s: %v", name, err)
	}

	fmt.Printf("PVC %s created successfully.\n", name)
	return nil
}

func (c *Client) DeletePVC(ctx context.Context, name string) error {
	err := c.client.CoreV1().PersistentVolumeClaims(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("PVC %s not found, nothing to delete.\n", name)
			return nil
		}
		return fmt.Errorf("failed to delete PVC %s: %v", name, err)
	}

	fmt.Printf("PVC %s deleted successfully.\n", name)
	return nil
}

func (c *Client) WatchPVC(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	watcher, err := c.client.CoreV1().PersistentVolumeClaims(c.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("error creating watcher: %v", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			pvc, ok := event.Object.(*corev1.PersistentVolumeClaim)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if pvc.Status.Phase == corev1.ClaimBound {
					return nil
				}
			case watch.Deleted:
				return fmt.Errorf("PVC %s was deleted", name)
			case watch.Error:
				return fmt.Errorf("error watching PVC")
			}

		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for PVC to be bound")
		}
	}
}
