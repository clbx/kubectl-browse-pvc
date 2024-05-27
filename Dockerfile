# PVCB Editor Dockerfile
FROM --platform=amd64 alpine

LABEL image="clbx/kubectl-browse-pvc" 
LABEL org.opencontainers.image.source = "https://github.com/clbx/kubectl-browse-pvc"

RUN apk update
RUN apk add vim bash shadow

COPY entrypoint.sh /entrypoint.sh
