#!/bin/bash

function exit_job {
    echo "exec closed"
    exit 0
}

trap exit_job SIGHUP

echo "PVCB job started, waiting for connection"

while :
do
    sleep 1
done 