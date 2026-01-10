package utils

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFindPodByPvc(t *testing.T) {
	tests := []struct {
		name string
		err  error

		client *fake.Clientset
	}{
		{
			name: "Single Pod,Single PVC",
			err:  nil,
			client: fake.NewClientset(
				&v1.PodList{
					Items: []v1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-pod",
								Namespace: "test-ns",
							},
							Spec: v1.PodSpec{
								Volumes: []v1.Volume{
									{
										Name: "test-volume",
										VolumeSource: v1.VolumeSource{
											PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
												ClaimName: "test-pvc",
											},
										},
									},
								},
							},
						},
					},
				},
				&v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pvc",
						Namespace: "test-ns",
					},
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			podList, err := test.client.CoreV1().Pods("test-ns").List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Errorf("Failed to list pods from client: %v", err)
			}
			pvc, err := test.client.CoreV1().PersistentVolumeClaims("test-ns").Get(ctx, "test-pvc", metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to list PVCs from client: %v", err)
			}
			pod := FindPodByPVC(*podList, *pvc)

			if pod.Name != "test-pod" {
				t.Errorf("Expected pod name to be 'test-pod', got '%s'", pod.Name)
			}
		})
	}
}
