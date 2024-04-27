package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/remotecommand"
)

var image string

func main() {

	var kubeConfigFlags = genericclioptions.NewConfigFlags(true)

	var rootCmd = &cobra.Command{
		Use:   "kubectl-browse-pvc",
		Short: "Kubernetes PVC Browser",
		Long:  `Kubernetes PVC Browser`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pvcName := args[0]
			getCommand(kubeConfigFlags, pvcName)
		},
	}

	rootCmd.Flags().StringVarP(&image, "image", "i", "clbx/pvcb-edit", "Image to mount job to")
	kubeConfigFlags.AddFlags(rootCmd.Flags())

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %s\n", err)
	}

}

func getCommand(kubeConfigFlags *genericclioptions.ConfigFlags, pvcName string) error {

	config, err := kubeConfigFlags.ToRESTConfig()
	if err != nil {
		log.Fatalf("Failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(*kubeConfigFlags.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get PVC: %v", err)
	}

	nsPods, err := clientset.CoreV1().Pods(*kubeConfigFlags.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to get pods: %v", err)
	}

	attachedPod := findPodByPVC(*nsPods, *targetPvc)

	manyAccessMode := false
	for _, mode := range targetPvc.Spec.AccessModes {
		if mode == corev1.ReadWriteMany || mode == corev1.ReadOnlyMany {
			manyAccessMode = true
			break
		}
	}

	if attachedPod == nil {
	} else {
		if manyAccessMode {
		} else {
			log.Fatalf("PVC attached to pod %s", attachedPod.Name)
		}
	}

	// Build the Job
	pvcbGetJob := buildPvcbGetJob(*kubeConfigFlags.Namespace, image, *targetPvc)
	// Create Job
	pvcbGetJob, err = clientset.BatchV1().Jobs(*kubeConfigFlags.Namespace).Create(context.TODO(), pvcbGetJob, metav1.CreateOptions{})

	if err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	timeout := 30

	for timeout > 0 {
		pvcbGetJob, err = clientset.BatchV1().Jobs(*kubeConfigFlags.Namespace).Get(context.TODO(), pvcbGetJob.GetObjectMeta().GetName(), metav1.GetOptions{})

		if err != nil {
			log.Fatalf("Failed to get job: %v", err)
		}

		if pvcbGetJob.Status.Active > 0 {
			break
		}

		time.Sleep(time.Second)

		timeout--
	}

	// Find the created pod.
	podList, err := clientset.CoreV1().Pods(*kubeConfigFlags.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "job-name=" + pvcbGetJob.Name,
	})

	if err != nil {
		log.Fatalf("Failed to get pods: %v", err)
	}

	if len(podList.Items) != 1 {
		fmt.Printf("%d\n", len(podList.Items))
		log.Fatalf("Found an unexpected number of controllers, this shouldn't happen.")
	}

	pod := &podList.Items[0]

	podSpinner := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	podSpinner.Suffix = " Waiting for Pod to Start\n"
	podSpinner.FinalMSG = "✓ Pod Started\n"
	podSpinner.Start()

	for pod.Status.Phase != corev1.PodRunning && timeout > 0 {

		pod, err = clientset.CoreV1().Pods(*kubeConfigFlags.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Failed to get pod: %v", err)
		}

		time.Sleep(time.Second)
		timeout--
	}

	podSpinner.Stop()
	if timeout == 0 {
		log.Fatalf("Pod failed to start")
	}

	req := clientset.CoreV1().RESTClient().
		Post().Resource("pods").
		Name(pod.Name).
		Namespace(*kubeConfigFlags.Namespace).
		SubResource("exec").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "true").
		Param("command", "bash").
		Param("command", "-c").
		Param("command", "cd /mnt && /bin/bash")

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Fatalf("Failed to create executor: %v", err)
	}

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to make raw terminal: %v", err)
	}

	defer terminal.Restore(int(os.Stdin.Fd()), oldState)

	ctx := context.Background()
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})

	if err != nil {
		log.Fatalf("Failed to stream: %v", err)
	}

	return nil

}
