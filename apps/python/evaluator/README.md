# Evaluator App

Is in charge of reading the Requester responses from the `responses` collection and calculate the tasks metrics to finally write them in the `metrics` collection.

### Deploy

1. Generate image
```bash
apps/python/evaluator/build.sh
```

2. Generate infrastructure following instructions in the [docker-compose README](../../../docker-compose/README.md)
