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

The URL of the Pocket RPC node should be added to `/etc/hosts` under the name `pokt.rpc.node.local`.

The private key of the application should be manually replaced in `tilt/apps/requester/local/patches/secret.yaml`, as a list under the field `apps`:
```json
...
"apps": [
        "6d7d9e78fd62b524cfa76a298b6f9653445449bc22960224901a5bb993ba52cb1802f4116b9d3798e2766a2452fbeb4d280fa99e77e61193df146ca4d88b38af"
      ],
...
```

### Troubleshot

It can happen that the deployment fails to start due to the `too many files open` error. To solve this execute:

```bash
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
```