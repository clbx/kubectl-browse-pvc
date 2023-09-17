package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
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

			if action != "mount" && action != "unmount" && action != "get" && action != "archive" {
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

			nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

			attachedPod := findPodByPVC(*nsPods, *targetPvc)

			if attachedPod == nil {
				fmt.Printf("Not attached to pod\n")
			} else {
				fmt.Printf("Attached to %s exiting.\n", attachedPod.Name)
			}

			if action == "get" {
				get(clientset, config, namespace, *targetPvc)
			}

			return nil

		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

func get(clientset *kubernetes.Clientset, config *rest.Config, namespace string, targetPvc corev1.PersistentVolumeClaim) error {

	// Build the Job
	pvcbGetJob := buildPvcbGetJob(namespace, targetPvc)
	// Create Job
	pvcbGetJob, err := clientset.BatchV1().Jobs(namespace).Create(context.TODO(), pvcbGetJob, metav1.CreateOptions{})

	if err != nil {
		panic(err)
	}

	timeout := 30

	// Wait 30 seconds for the Job to start.
	for timeout > 0 {
		pvcbGetJob, err = clientset.BatchV1().Jobs(namespace).Get(context.TODO(), pvcbGetJob.GetObjectMeta().GetName(), metav1.GetOptions{})

		if err != nil {
			panic(err.Error())
		}

		if pvcbGetJob.Status.Active > 0 {
			fmt.Printf("Job is running \n")
			break
		}

		fmt.Printf("Not started yet")
		time.Sleep(time.Second)

		timeout--
	}

	// Find the created pod.
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "job-name=" + pvcbGetJob.Name,
	})

	if err != nil {
		panic(err.Error())
	}

	if len(podList.Items) != 1 {
		fmt.Printf("%d\n", len(podList.Items))
		panic("Found more or less than one pod")
	}

	pod := &podList.Items[0]

	for pod.Status.Phase != corev1.PodRunning && timeout > 0 {
		fmt.Printf("Waiting for pod. Status: %s\n", pod.Status.Phase)

		pod, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}

		time.Sleep(time.Second)
		timeout--
	}

	if timeout == 0 {
		panic("Pod failed to start")
	}

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(namespace).
		SubResource("exec").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "true").
		Param("command", "/bin/bash")

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err.Error())
	}

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	defer terminal.Restore(int(os.Stdin.Fd()), oldState)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})

	if err != nil {
		panic(err.Error())
	}

	return nil

}
