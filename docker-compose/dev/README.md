# Development Environment

This folder contains the Docker-Compose description files used to deploy the Pocket Network ML Test-Bench for development.
If you want to deploy the test-bench in production we recommend to use another approach for better scalability (like Kubernetes).

In order to test the project, two modules set of containers are required: the temporal apps and their dependencies.

## Dependencies

This folder contains all the dependencies used by the bench, it includes:
- MongoDB : Used for App communication and node register.
- PostgreSQL : Used by Temporal IO and the Sampler App for dataset storing.
- Temporal IO : Used to coordinate Apps execution.
Besides those core services, two UIs are also provided:
- Temporal UI : To observe and manage workflow execution.
- Pgadmin : To inspect the PostgreSQL database.

To deploy all these container just execute:
```bash
cd dependencies && docker compose up -d
```
To shut them down just do (omit the `-v` if you want to keep the DBs and Temporal data):
```bash
cd dependencies && docker compose down -v
```

Once all services are healthy you can access the PGadmin on [http://localhost:5050/](http://localhost:5050/) using:
- User: `admin@local.dev`
- Password: `admin`

Note: to change these values edit the `./dependencies/.env` file.

The Temporal UI is available at [http://localhost:8080](http://localhost:8080).
Finally, you can connect to the MongoDB using the following connection string (and a tool like MongoDB Compass):
`mongodb://127.0.0.1:27017/?replicaSet=devRs`

In order to have quicker access to the Temporal client, create an alias:
```bash
alias temporal_docker="docker exec temporal-admin-tools temporal"
```

Then create the namespace for the test-bench (this is not necessary, it is already created):
```bash
temporal_docker operator namespace create pocket-ml-testbench
```


## Test-Bench Apps

To deploy the Apps, which control the bench execution consuming tasks from the Temporal IO queues, you need to deploy the containers described in the compose file in the `apps` folder. To deploy them do:
```bash
cd apps && docker compose up -d
```
To shut them down:
```bash
cd apps && docker compose down -v
```

The deployment configuration can be found at `./apps/config`.

## Create Requester Scheduled Workflow for each application/service you want to control.

This will create a scheduled workflow that will run every 3 minutes with a timeout close to 3m
```
temporal_docker schedule create \
    --schedule-id 'f3abbe313689a603a1a6d6a43330d0440a552288-0001' \
    --workflow-id 'f3abbe313689a603a1a6d6a43330d0440a552288-0001' \
    --namespace pocket-ml-testbench \
    --workflow-type 'requester' \
    --task-queue 'requester' \
    --cron '@every 3m' \
    --execution-timeout 175 \
    --overlap-policy 'BufferOne' \
    --input '{"app":"f3abbe313689a603a1a6d6a43330d0440a552288","service":"0001"}'
```
Later you can pause/trigger/cancel this from Temporal CLI or [Temporal UI](http://localhost:8080/namespaces/pocket-ml-testbench/schedules)