package main

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodOptions struct {
	image     string
	namespace string
	pvc       corev1.PersistentVolumeClaim
	cmd       []string
	args      []string
}

var script = `
chmod +x /etc/profile.d/ps1.sh

base_processes=$(ls /proc | grep -E '^[0-9]+$' | while read -r pid; do cat /proc/"$pid"/comm 2>/dev/null; done | grep -E "ash|bash|sh" | wc -l)
echo "Processes: $base_processes"
sleep 2

while :; do
    shell_processes=$(ls /proc | grep -E '^[0-9]+$' | while read -r pid; do cat /proc/"$pid"/comm 2>/dev/null; done | grep -E "ash|bash|sh" | wc -l)
    if [ "$shell_processes" -gt "$base_processes" ]; then
        echo "Found an additional process"
        while [ "$shell_processes" -gt "$base_processes" ]; do
            sleep 2
            shell_processes=$(ls /proc | grep -E '^[0-9]+$' | while read -r pid; do cat /proc/"$pid"/comm 2>/dev/null; done | grep -E "ash|bash|sh" | wc -l)
        done
        exit 0
    fi 
done
`

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
// func buildPvcbGetJob(namespace string, image string, pvc corev1.PersistentVolumeClaim) *batchv1.Job {
func buildPvcbGetJob(options PodOptions) *batchv1.Job {

	//Check if provided arguments is empty. If so use the browsing script
	if len(options.args) == 0 {
		options.args = []string{script}
	}

	TTLSecondsAfterFinished := new(int32)
	*TTLSecondsAfterFinished = 10

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "browse-" + options.pvc.Name,
			Namespace: options.namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "browse-pvc",
					Labels: map[string]string{
						"job-name": "browse-" + options.pvc.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:    "browser",
							Image:   image,
							Command: options.cmd,
							Args:    options.args,
							Env: []corev1.EnvVar{
								{
									Name:  "PS1",
									Value: "\\h:\\w\\$ ",
								},
							},
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
									ClaimName: options.pvc.Name,
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
