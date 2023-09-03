package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

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
				fmt.Printf("Error: No PVC Defined\n")
				os.Exit(1)
			}

			targetPvcName := cCtx.Args().Get(0)

			targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), targetPvcName, metav1.GetOptions{})

			if err != nil {
				fmt.Printf("%s\n", err)
				os.Exit(1)
			}

			fmt.Printf("%s\n", targetPvc.Name)

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
