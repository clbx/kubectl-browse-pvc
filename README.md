# pvc-browser
Tool to browse a Kubernetes PVC from the command line

This tool is WIP, but currently can connect to a non-mounted PVC
```
pvcb get <pvc name> -n <namespace>
```

## Desired Features
- [ ] Mount PVCs to a directory
- [ ] Create method to transfer files
- [ ] Deal with PVCs that are mounted, scale down or otherwise make the PVC available
- [ ] Handle different Access modes
