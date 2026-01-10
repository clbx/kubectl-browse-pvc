package utils

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodOptions struct {
	Image       string
	Namespace   string
	Pvc         corev1.PersistentVolumeClaim
	Cmd         []string
	Args        []string
	Node        string
	User        int64
	Tolerations []corev1.Toleration
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
func FindPodByPVC(podList corev1.PodList, pvc corev1.PersistentVolumeClaim) *corev1.Pod {
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
func BuildPvcbGetJob(options PodOptions) *batchv1.Job {

	//Check if provided arguments is empty. If so use the browsing script
	if len(options.Args) == 0 {
		options.Args = []string{script}
	}

	// Setup SecurityContext
	var allowPrivilegeEscalation bool
	var runAsNonRoot bool
	var capabilities corev1.Capabilities
	if options.User == 0 {
		runAsNonRoot = false
		allowPrivilegeEscalation = true
		capabilities = corev1.Capabilities{
			Add:  []corev1.Capability{"CHOWN", "FOWNER"},
			Drop: []corev1.Capability{"ALL"},
		}
	} else {
		runAsNonRoot = true
		allowPrivilegeEscalation = false
		capabilities = corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
	}

	securityContext := corev1.SecurityContext{
		RunAsUser:                &options.User,
		RunAsNonRoot:             &runAsNonRoot,
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities:             &capabilities,
		SeccompProfile: &corev1.SeccompProfile{
			Type: "RuntimeDefault",
		},
	}

	TTLSecondsAfterFinished := new(int32)
	*TTLSecondsAfterFinished = 10

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "browse-" + options.Pvc.Name,
			Namespace: options.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "browse-pvc",
					Labels: map[string]string{
						"job-name": "browse-" + options.Pvc.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					NodeName:      options.Node,
					Containers: []corev1.Container{
						{
							Name:            "browser",
							Image:           options.Image,
							Command:         options.Cmd,
							Args:            options.Args,
							SecurityContext: &securityContext,
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
					Tolerations: options.Tolerations,
					Volumes: []corev1.Volume{
						{
							Name: "target-pvc",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: options.Pvc.Name,
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
