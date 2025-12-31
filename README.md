# kubectl-browse-pvc

Kubectl plugin to browse a Kubernetes PVC from the command line

I constantly found myself spinning up dummy pods to exec into them so I could browse a PVC, this takes a few steps out of creating dummy pods to check out the contents of a PVC.

## Installation

Install via krew

```sh
kubectl krew install browse-pvc
```

## Usage

```sh
kubectl browse-pvc <pvc-name>
```

On a PVC. The tool spins up a pod that mounts the PVC and then execs into it allowing you to modify the contents of the PVC. The Job finishes and cleans up the pod when you disconnect.

Commands can be described to run a command instead of popping a shell

```sh
kubectl browse-pvc <pvc-name> -- <command>
```

A User ID can be described to set the user the container runs as

```sh
kubectl browse-pvc -u 1000 <pvc-name>
```

### Configuring auto-completion

```sh
cat > kubectl_browse-pvc <<EOF
#!/usr/bin/env sh

# Call the __complete command passing it all arguments
kubectl browse-pvc __complete "\$@"
EOF

chmod +x kubectl_browse-pvc

# Important: the following command may require superuser permission
sudo mv kubectl_browse-pvc /usr/local/bin
```

## Dev

### Test

#### All Tests

```sh
go test -v ./...
```

#### Specific Modules

```sh
go test -v github.com/clbx/kubectl-browse-pvc/<MODULE>
```

Example: `go test -v github.com/clbx/kubectl-browse-pvc/internal/utils`

### Build

```sh
go build -v -o kubectl-browse-pvc cmd/browse-pvc/main.go
```
