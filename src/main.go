package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"k8s.io/client-go/kubernetes"
)

var image string
var Version string

func main() {

	var kubeConfigFlags = genericclioptions.NewConfigFlags(true)

	if Version == "" {
		Version = "devel"
	}

	var rootCmd = &cobra.Command{
		Use:     "kubectl-browse-pvc",
		Short:   "Kubernetes PVC Browser",
		Long:    `Kubernetes PVC Browser`,
		Version: Version,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pvcName := args[0]
			browseCommand(kubeConfigFlags, pvcName)
		},
	}

	rootCmd.Flags().StringVarP(&image, "image", "i", "alpine", "Image to mount job to")
	kubeConfigFlags.AddFlags(rootCmd.Flags())

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %s\n", err)
	}

}

func browseCommand(kubeConfigFlags *genericclioptions.ConfigFlags, pvcName string) error {

	config, err := kubeConfigFlags.ToRESTConfig()
	if err != nil {
		log.Fatalf("Failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	// Get namespace if not set
	namespace := *kubeConfigFlags.Namespace
	if namespace == "" {
		config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			log.Fatalf("Failed to load kubeconfig: %v", err)
		}
		namespace = config.Contexts[config.CurrentContext].Namespace
		if err != nil {
			log.Fatalf("Failed to get namespace from current context: %v", err)
		}
	}

	targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get PVC: %v", err)
	}

	nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
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

	options := &PodOptions{
		image:     image,
		namespace: namespace,
		pvc:       *targetPvc,
		cmd:       []string{"/bin/sh", "-c", "--"},
	}

	// Build the Job
	pvcbGetJob := buildPvcbGetJob(*options)
	// Create Job
	pvcbGetJob, err = clientset.BatchV1().Jobs(namespace).Create(context.TODO(), pvcbGetJob, metav1.CreateOptions{})

	if err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	timeout := 30

	for timeout > 0 {
		pvcbGetJob, err = clientset.BatchV1().Jobs(namespace).Get(context.TODO(), pvcbGetJob.GetObjectMeta().GetName(), metav1.GetOptions{})

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
	podSpinner.FinalMSG = "✓ Attached to " + pod.Name + "\n"
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

	request := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(options.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"sh", "-c", "cd /mnt && (ash || bash || sh)"},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", request.URL())
	if err != nil {
		return err
	}

	oldState, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}
	defer term.Restore(0, oldState)

	terminalSizeQueue := &sizeQueue{
		resizeChan:   make(chan remotecommand.TerminalSize, 1),
		stopResizing: make(chan struct{}),
	}

	// prime with initial term size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil {
		terminalSizeQueue.resizeChan <- remotecommand.TerminalSize{Width: uint16(width), Height: uint16(height)}
	}

	go terminalSizeQueue.MonitorSize()
	defer terminalSizeQueue.Stop()

	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		Tty:               true,
		TerminalSizeQueue: terminalSizeQueue,
	})
	if err != nil {
		return err
	}

	return nil
}
