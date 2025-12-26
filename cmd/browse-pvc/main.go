package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/clbx/kubectl-browse-pvc/internal/monitor"
	"github.com/clbx/kubectl-browse-pvc/internal/utils"
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
var containerUser int

var validCommands = []string{"image", "container-user"}

func main() {

	var kubeConfigFlags = genericclioptions.NewConfigFlags(true)

	if Version == "" {
		Version = "devel"
	}

	var rootCmd = &cobra.Command{
		Use:       "kubectl-browse-pvc <PVC-NAME> [-- COMMAND [ARGS...]]",
		Short:     "Kubernetes PVC Browser",
		Long:      `Kubernetes PVC Browser`,
		Version:   Version,
		Args:      cobra.MinimumNArgs(1),
		ValidArgs: validCommands,
		Run: func(cmd *cobra.Command, args []string) {
			pvcName := args[0]
			commandArgs := args[1:]
			browseCommand(kubeConfigFlags, pvcName, commandArgs)
		},
	}

	rootCmd.Flags().StringVarP(&image, "image", "i", "alpine", "Image to mount job to")
	rootCmd.Flags().IntVarP(&containerUser, "container-user", "u", 0, "User ID to run the container as")
	kubeConfigFlags.AddFlags(rootCmd.Flags())

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %s\n", err)
	}

}

func browseCommand(kubeConfigFlags *genericclioptions.ConfigFlags, pvcName string, commandArgs []string) error {

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

	fmt.Printf("Browsing PVC %s in namespace %s\n", pvcName, namespace)

	targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get PVC: %v", err)
	}

	nsPods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to get pods: %v", err)
	}

	attachedPod := utils.FindPodByPVC(*nsPods, *targetPvc)

	manyAccessMode := false
	for _, mode := range targetPvc.Spec.AccessModes {
		if mode == corev1.ReadWriteMany || mode == corev1.ReadOnlyMany {
			manyAccessMode = true
			break
		}
	}

	var node string = ""

	// If the pvc is a rwo and already attached to a node, it needs to scheduled to the same node as the workload
	if attachedPod != nil && !manyAccessMode {
		node = attachedPod.Spec.NodeName
	}

	options := &utils.PodOptions{
		Image:     image,
		Namespace: namespace,
		Pvc:       *targetPvc,
		Cmd:       []string{"/bin/sh", "-c", "--"},
		Args:      commandArgs,
		Node:      node,
		User:      int64(containerUser),
	}

	// Build the Job
	pvcbGetJob := utils.BuildPvcbGetJob(*options)
	// Create Job
	pvcbGetJob, err = clientset.BatchV1().Jobs(namespace).Create(context.TODO(), pvcbGetJob, metav1.CreateOptions{})

	//Handle if there were command args
	if len(commandArgs) > 0 {
		waitForJobCompletion(*clientset, namespace, pvcbGetJob.Name)
		logs, err := getPodLogs(clientset, namespace, targetPvc.Name)
		if err != nil {
			log.Fatalf("Failed to get logs: %v", err)
		}
		fmt.Printf("%s", logs)
		return nil
	}

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
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
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

		pod, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
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
		Namespace(options.Namespace).
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

	terminalSizeQueue := &monitor.SizeQueue{
		ResizeChan:   make(chan remotecommand.TerminalSize, 1),
		StopResizing: make(chan struct{}),
	}

	// prime with initial term size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil {
		terminalSizeQueue.ResizeChan <- remotecommand.TerminalSize{Width: uint16(width), Height: uint16(height)}
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

func waitForJobCompletion(clientset kubernetes.Clientset, namespace string, jobName string) error {
	timeout := 300
	for timeout > 0 {
		job, err := clientset.BatchV1().Jobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			return nil
		}
		time.Sleep(time.Second)
		timeout--
	}
	return fmt.Errorf("job failed to complete within 300s")
}

func getPodLogs(clientset *kubernetes.Clientset, namespace string, pvcName string) (string, error) {

	//find the pod based on the label applied to the pod in the job
	labelSelector := fmt.Sprintf("job-name=browse-%s", pvcName)
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return "", fmt.Errorf("failed to get pods: %v", err)
	}

	if len(podList.Items) != 1 {
		return "", fmt.Errorf("found an unexpected number of pods with the browse label, this shouldn't happen")
	}

	podName := podList.Items[0].Name

	podLogOpts := corev1.PodLogOptions{}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())

	if err != nil {
		return "", fmt.Errorf("failed to get logs: %v", err)
	}

	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to copy logs: %v", err)
	}
	return buf.String(), nil
}
