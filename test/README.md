# Testing Resources

Kubernetes manifests for testing access

- ```bound-pvc``` is a pvc bound to a pod, browse-pvc should fail to connect and return an erro
- ``failed-pvc`` a pvc that is not bound to a pod, but is in some kind of failure/pending state where it cannot be mounted.
- ``rwx-bound-pvc`` a RWX pvc that is attached to another pod
- ``rwx-unbound-pvc`` A RW pvc that is not attached to another pod
- ``unbound-pvc`` a pvc that is not connected to any other pod