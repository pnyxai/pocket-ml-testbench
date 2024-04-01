Create a command alias for the Temporal CLI:
`alias temporal_docker="docker exec temporal-admin-tools temporal"`
Create namespace
`temporal_docker operator namespace create tester_namespace`


Start a workflow for chain X en app 1
```
temporal_docker workflow start \
 --task-queue relay-tester-local \
 --type RelaySampler \
 --input '"555-55-5555"' \
 --namespace tester_namespace \
 --workflow-id relay_test_workflow
```

Start a workflow for chain Y en app 1
```
temporal_docker workflow start \
 --task-queue relay-tester-local \
 --type RelaySampler \
 --input '"555-55-5555"' \
 --namespace tester_namespace \
 --workflow-id relay_test_workflow
```