# kubectl-browse-pvc
Kubectl plugin to browse a Kubernetes PVC from the command line

I constantly found myself spinning up dummy pods to exec into them so I could browse a PVC, this takes a few steps out of creating dummy pods to check out the contents of a PVC. 

Install via krew
```
kubectl krew install browse-pvc
```

Usage

```
kubectl browse-pvc -n <namespace> <pvc-name>
```
On an unbound PVC. The tool spins up a pod that mounts the PVC and then execs into it allowing you to modify the contents of the PVC. The Job finishes and cleans up the pod when you disconnect.
