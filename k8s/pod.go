package k8s

import (
	"context"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Create(clientset *kubernetes.Clientset, namespace string) (*core.Pod, error) {
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: "streamer-",
			Labels: map[string]string{
				"app": "streamer",
			},
		},
		Spec: core.PodSpec{
			NodeSelector: map[string]string{
				"cloud.google.com/compute-class": "Balanced",
			},
			Containers: []core.Container{
				{
					Name:            "streamer",
					Image:           "ghcr.io/codecflow/conductor:1.0.0",
					ImagePullPolicy: core.PullIfNotPresent,
					// Args:            []string{"tail", "-f", "/dev/null"},
					Ports: []core.ContainerPort{
						{
							ContainerPort: 9222,
						},
					},
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							"cpu":               resource.MustParse("4"),
							"memory":            resource.MustParse("8Gi"),
							"ephemeral-storage": resource.MustParse("2Gi"),
						},
						Requests: core.ResourceList{
							"cpu":               resource.MustParse("2"),
							"memory":            resource.MustParse("4Gi"),
							"ephemeral-storage": resource.MustParse("1Gi"),
						},
					},
					Env: []core.EnvVar{
						{
							Name:  "URL",
							Value: "https://www.youtube.com/watch?v=I9wJNRfo1pk",
						},
						{
							Name:  "RTMP",
							Value: "rtmp://ingest.global-contribute.live-video.net/app/live_1210610726_abc1ufMSJQ9F47o4ojIkg64rztYcV5",
						},
						{
							Name:  "DISPLAY",
							Value: ":99",
						},
						{
							Name:  "OVERLAY",
							Value: ":98",
						},
						{
							Name:  "WIDTH",
							Value: "1920",
						},
						{
							Name:  "HEIGHT",
							Value: "1080",
						},
					},
				},
			},
			ImagePullSecrets: []core.LocalObjectReference{
				{
					Name: "ghcr",
				},
			},
		},
	}

	return clientset.CoreV1().Pods(namespace).Create(context.TODO(), pod, meta.CreateOptions{})
}
