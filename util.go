package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Gets the controller that controls a pod to scale it down.
func getPodController(clientset *kubernetes.Clientset, config *rest.Config, namespace string, pod corev1.Pod) (interface{}, error) {

	fmt.Printf("%+v\n", &pod.OwnerReferences)

	if len(pod.OwnerReferences) == 0 {
		return nil, errors.New("No Owner References Found")
	}

	if len(pod.OwnerReferences) > 1 {
		return nil, errors.New("Unable to handle multiple owner references.")
	}

	kind := pod.OwnerReferences[0].Kind
	name := pod.OwnerReferences[0].Name

	switch kind {
	//ReplicaSet Deployment StatefulSet DaemonSet Job CronJob
	case "ReplicaSet":
		return clientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case "Deployment":
		return clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case "StatefulSet":
		return clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case "DaemonSet":
		return clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case "Job":
		return clientset.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case "CronJob":
		return clientset.BatchV1().CronJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	default:
		return nil, errors.New("Unknown controller type")
	}

}

// Finds if a pod that attached to a PVC
func findPodByPVC(podList corev1.PodList, pvc corev1.PersistentVolumeClaim) *corev1.Pod {
	for _, pod := range podList.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return &pod
			}
		}
	}
	return nil
}

// Returns a job for the get command.
func buildPvcbGetJob(namespace string, pvc corev1.PersistentVolumeClaim) *batchv1.Job {

	TTLSecondsAfterFinished := new(int32)
	*TTLSecondsAfterFinished = 10

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvcb-edit-" + pvc.Name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvcb-edit",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:    "pvcb-edit",
							Image:   "clbx/pvcb-edit",
							Command: []string{"/bin/bash", "-c", "--"},
							Args:    []string{"/entrypoint.sh"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "target-pvc",
									MountPath: "/mnt",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "target-pvc",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	return job
}

func getClientSetFromKubeconfig() (*kubernetes.Clientset, *rest.Config) {
	var kubeconfig string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		fmt.Println("error: unable to locate kubeconfig")
		os.Exit(1)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset, config
}
