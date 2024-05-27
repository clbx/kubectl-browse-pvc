#!/bin/bash

base_processes=$(pgrep -l bash | wc -l)
echo "Processes: $base_processes"
sleep 2

while :; do
    bash_processes=$(pgrep -l bash | wc -l)
    if [ $bash_processes -gt $base_processes ]; then
        echo "Found an additional process"
        while [ $bash_processes -gt $base_processes ]; do
            sleep 2
            bash_processes=$(pgrep -l bash | wc -l)
        done
    exit 0
    fi 
done
