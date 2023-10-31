package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func handleScaleDown(attachedPod *corev1.Pod, clientset *kubernetes.Clientset) error {
	fmt.Printf("PVC already attached to %s.\n", attachedPod.Name)
	controller, err := getPodController(clientset, *attachedPod)

	if err != nil {
		return err
	}

	switch obj := controller.(type) {
	case *appsv1.ReplicaSet:
		isControlledByDeployment := false
		// Usually ReplicaSets are controlled by a Deployment.
		ownerRefs := obj.GetOwnerReferences()

		for _, ownerRef := range ownerRefs {
			if ownerRef.Kind == "Deployment" {
				isControlledByDeployment = true
				break
			}
		}
		owningDeployment, err := clientset.AppsV1().Deployments(obj.Namespace).Get(context.TODO(), ownerRefs[0].Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if isControlledByDeployment {
			scaleDeployment(*owningDeployment, true, 0)
		} else {
			fmt.Printf("%s is controlled by ReplicaSet %s.\n", attachedPod.Name, obj.Name)
			//scaleReplicaSet()
		}
	case *appsv1.Deployment:
		fmt.Printf("%s is controlled by Deployment %s.\n", attachedPod.Name, obj.Name)
		scaleDeployment(*obj, true, 0)
	case *appsv1.StatefulSet:
		fmt.Printf("%s is controlled by StatefulSet %s.", attachedPod.Name, obj.Name)
		//scaleStatefulSet()
	default:
		return errors.New("Unknown/Unsupported Controller Type")
	}

	return nil

}

func handleScaleUp() {

}

func scaleDeployment(deployment appsv1.Deployment, prompt bool, target int32) error {
	var input string
	fmt.Printf("Scale down %s? (y/n) ", deployment.Name)
	_, err := fmt.Scanln(&input)
	if err != nil {
		return err
	}

	if strings.ToLower(input) == "y" {
		fmt.Printf("Scaling down %s.\n", deployment.Name)
		deployment.Spec.Replicas = &target
		return nil
	} else {
		return errors.New("declined scaling")
	}

}
