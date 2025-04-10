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
# List of Pocket Network RPCs, used by the Manager and Requester
POKT_RPC_LIST=["http://pokt.rpc.node.local:9081"]
# List of private keys of the Pocket Network apps used by the Requester
APPS_PRIVATE_KEYS_LIST=["6d7d9e78fd62b524cfa76a298b6f9653445449bc22960224901a5bb993ba52cb1802f4116b9d3798e2766a2452fbeb4d280fa99e77e61193df146ca4d88b38af"]
```

These values will be replaced in all `*.template.yaml`. 

### Troubleshot

It can happen that the deployment fails to start due to the `too many files open` error. To solve this execute:

```bash
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
```