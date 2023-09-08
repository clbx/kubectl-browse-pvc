package main

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func buildPvcbJob(namespace string, pvc corev1.PersistentVolumeClaim) *batchv1.Job {

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
					Image:   "ghcr.io/clbx/pvcb-edit",
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
