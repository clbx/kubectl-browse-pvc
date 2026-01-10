package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Retrieves a given node's taints
func GetNodeTaints(clientset kubernetes.Interface, nodeName string) ([]corev1.Taint, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return node.Spec.Taints, nil
}

// Returns the tolerations necessary for the taints provided
func BuildTolerationsForTaints(taints []corev1.Taint) []corev1.Toleration {

	tolerations := make([]corev1.Toleration, len(taints))

	for index, taint := range taints {
		tmpTol := corev1.Toleration{
			Key:      taint.Key,
			Value:    taint.Value,
			Operator: corev1.TolerationOpEqual,
			Effect:   taint.Effect,
		}
		tolerations[index] = tmpTol
	}

	return tolerations
}
