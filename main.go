package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh/terminal"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/client-go/tools/remotecommand"
)

func main() {

	var namespace string
	var image string

	app := &cli.App{
		Name:  "kubectl browse-pvc",
		Usage: "Kubernetes PVC Browser",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "namespace",
				// Set the default to the current context instead of default
				Value:       "default",
				Usage:       "Specify namespace of ",
				Aliases:     []string{"n"},
				Destination: &namespace,
			},
			&cli.StringFlag{
				Name: "image",
				//use the pvc browser edit container
				Value:       "clbx/pvcb-edit",
				Usage:       "Image to mount job to",
				Aliases:     []string{"i"},
				Destination: &image,
			},
		},
		Action: getCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

func getCommand(c *cli.Context) error {

	if c.Args().Len() == 0 {
		cli.ShowAppHelp(c)
		return cli.NewExitError("", 1)
	}

	clientset, config := getClientSetFromKubeconfig()

	targetPvcName := c.Args().Get(0)
	targetPvc, err := clientset.CoreV1().PersistentVolumeClaims(c.String("namespace")).Get(context.TODO(), targetPvcName, metav1.GetOptions{})

	if err != nil {
		return cli.NewExitError(err, 1)
	}

	nsPods, err := clientset.CoreV1().Pods(c.String("namespace")).List(context.TODO(), metav1.ListOptions{})
	attachedPod := findPodByPVC(*nsPods, *targetPvc)

	if attachedPod == nil {
	} else {
		errMsg := fmt.Sprintf("PVC attached to pod %s", attachedPod.Name)
		return cli.NewExitError(errMsg, 1)
	}

	// Build the Job
	pvcbGetJob := buildPvcbGetJob(c.String("namespace"), c.String("image"), *targetPvc)
	// Create Job
	pvcbGetJob, err = clientset.BatchV1().Jobs(c.String("namespace")).Create(context.TODO(), pvcbGetJob, metav1.CreateOptions{})

	if err != nil {
		return cli.NewExitError(err, 1)
	}

	timeout := 30

	for timeout > 0 {
		pvcbGetJob, err = clientset.BatchV1().Jobs(c.String("namespace")).Get(context.TODO(), pvcbGetJob.GetObjectMeta().GetName(), metav1.GetOptions{})

		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if pvcbGetJob.Status.Active > 0 {
			break
		}

		time.Sleep(time.Second)

		timeout--
	}

	// Find the created pod.
	podList, err := clientset.CoreV1().Pods(c.String("namespace")).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "job-name=" + pvcbGetJob.Name,
	})

	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if len(podList.Items) != 1 {
		fmt.Printf("%d\n", len(podList.Items))
		return cli.NewExitError("Found an unexpected number of controllers, this shouldn't happen.", 1)
	}

	pod := &podList.Items[0]

	podSpinner := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	podSpinner.Suffix = " Waiting for Pod to Start\n"
	podSpinner.FinalMSG = "✓ Pod Started\n"
	podSpinner.Start()

	for pod.Status.Phase != corev1.PodRunning && timeout > 0 {

		pod, err = clientset.CoreV1().Pods(c.String("namespace")).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		time.Sleep(time.Second)
		timeout--
	}

	podSpinner.Stop()
	if timeout == 0 {
		return cli.NewExitError("Pod failed to start", 1)
	}

	req := clientset.CoreV1().RESTClient().
		Post().Resource("pods").
		Name(pod.Name).
		Namespace(c.String("namespace")).
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
		return cli.NewExitError(err, 1)
	}

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer terminal.Restore(int(os.Stdin.Fd()), oldState)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})

	if err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil

}
