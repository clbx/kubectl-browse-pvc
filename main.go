package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {

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

	var namespace string

	app := &cli.App{
		Name:  "pvcb",
		Usage: "Kubernetes PVC Browser",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "namespace",
				Value:       "default",
				Usage:       "Specify namespace of ",
				Aliases:     []string{"n"},
				Destination: &namespace,
			},
		},
		Action: func(cCtx *cli.Context) error {

			if cCtx.Args().Len() == 0 {
				fmt.Printf("Error: No action defined\n")
				os.Exit(1)
			}

			action := cCtx.Args().Get(0)

			if action != "mount" && action != "unmount" && action != "edit" && action != "archive" {
				fmt.Printf("%s is not a valid action\n", action)
				os.Exit(1)
			}

			targetPvcName := cCtx.Args().Get(1)

			if targetPvcName == "" {
				fmt.Printf("Error: No PVC defined\n")
				os.Exit(1)
			}

			targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), targetPvcName, metav1.GetOptions{})

			if err != nil {
				fmt.Printf("%s\n", err)
				os.Exit(1)
			}

			// Check pods to see if PVC is attached

			nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

			attachedPod := findPodByPVC(*nsPods, *targetPvc)

			// check different setups (atttached, accessmodes, etc.)
			if attachedPod == nil {
				fmt.Printf("Not attached to pod\n")
			} else {
				fmt.Printf("Attached to %s exiting.\n", attachedPod.Name)
			}

			pvcbPod := buildPvcbPod(namespace, *targetPvc)

			pvcbPod, err = clientset.CoreV1().Pods("default").Create(context.TODO(), pvcbPod, metav1.CreateOptions{})
			if err != nil {
				panic(err.Error())
			}

			fmt.Printf("Pod %s created\n", pvcbPod.ObjectMeta.Name)

			timeout := 30

			for pvcbPod.Status.Phase != corev1.PodRunning && timeout > 0 {
				fmt.Printf("Waiting for pod.\n")
				time.Sleep(time.Second)
				timeout--
			}

			if timeout == 0 {
				fmt.Printf("Pod failed to start in \n")
				os.Exit(1)
			}

			fmt.Printf("Pod started up ")

			// If its not attached to a pod, deploy the pvcb image into the namespace and mount it

			//Find if PVC is mounted to a pod.
			//nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

			// for _, pvc := range pvcs.Items {
			// 	fmt.Printf("%s\n", pvc.Name)
			// }

			// // Check to see if the pvc is attached to a pod
			// nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
			// var attachedPod corev1.Pod

			// fmt.Printf("%s", attachedPod.Name)

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

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

func buildPvcbPod(namespace string, pvc corev1.PersistentVolumeClaim) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvcb-edit",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "pvcb-edit",
					Image:   "debian",
					Command: []string{"bin/bash", "-c", "--"},
					Args:    []string{"while true; do sleep 30; done;"},
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
	}

	return pod

}
