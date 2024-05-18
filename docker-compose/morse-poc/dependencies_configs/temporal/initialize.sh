#!/bin/bash

temporal operator namespace create pocket-ml-testbench

sleep 5

temporal schedule create \
    --schedule-id 'lmeh-00A1-poc' \
    --workflow-id 'lmeh-00A1-poc' \
    --namespace 'pocket-ml-testbench' \
    --workflow-type 'Manager' \
    --task-queue 'manager' \
    --cron '@every 5m' \
    --execution-timeout 175 \
    --overlap-policy 'BufferOne' \
    --input '{"service":"00A1", "tests": [{"framework": "lmeh", "tasks": ["mmlu_high_school_macroeconomics"]}]}'

sleep 5

temporal schedule create \
    --schedule-id 'f3abbe313689a603a1a6d6a43330d0440a552288-00A1-poc' \
    --workflow-id 'f3abbe313689a603a1a6d6a43330d0440a552288-00A1-poc' \
    --namespace 'pocket-ml-testbench' \
    --workflow-type 'Requester' \
    --task-queue 'requester' \
    --cron '@every 5m' \
    --execution-timeout 175 \
    --overlap-policy 'BufferOne' \
    --input '{"app":"f3abbe313689a603a1a6d6a43330d0440a552288","service":"00A1"}'