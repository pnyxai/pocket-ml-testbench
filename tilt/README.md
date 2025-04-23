# Development Environment with TILT

To deploy the development environment you will need a local k8s cluster, we recommend [KIND](https://kind.sigs.k8s.io/).
Then you will need to install [TILT](https://docs.tilt.dev/), depending on you OS, this will vary.

To deploy just execute:

```bash
tilt up
```

then visit [127.0.0.1:10350](127.0.0.1:10350) and wait until all services are green.

To delete the deployment just execute:

```bash
tilt down
```

### External Services

To deploy the dev env you will need to have access to:
- A Pocket Node for RPC calls
- One (or more) Pocket Application private key (staked in the used services)

This data should be added to a `.env` file that must be at the same level of the `Tiltfile`, we provide a sample of that file, but you will need to change the values:
```dotenv
# POKT Network RPCs
POKT_RPC="http://127.0.0.1:26657"
POKT_GRPC="127.0.0.1:9090"
# Pockent Network Apps used for relaying
APPS_LIST={"app_address" : "app_pk_hex", "app_address" : "app_pk_hex"}
# Services to watch and the associated app addresses 
APPS_PER_SERVICE="<service id>=<app address>, <service id>=<app address>"
# Huggingface token, for dataset downloading
HF_TOKEN="YOUR TOKEN"
```

These values will be replaced in all `*.template.yaml`. 

### Troubleshot

It can happen that the deployment fails to start due to the `too many files open` error. To solve this execute:

```bash
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
```