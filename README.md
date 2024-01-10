# kubectl-browse-pvc
Kubectl plugin to browse a Kubernetes PVC from the command line

Install via krew
```
kubectl krew install browse-pvc
```

Usage

```
kubectl browse-pvc -n <namespace> <pvc-name>
```
On an unbound PVC. The tool spins up a pod that mounts the PVC and then execs into it allowing you to modify the contents of the PVC. The Job finishes and cleans up the pod when you disconnect.
