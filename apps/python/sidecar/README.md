# POKT LLM Sidecar

This is a minimalist sidecar application that is deployed alongside the LLM nodes. It was created to have access to the served language model tokenizer that is not exposed by OpenAI API but we need to have it in a decentralized environment such as POKT.

This sidecar will use the provided configuration to load the LLM tokenizer and serve its data on the `/pokt` path of the node. Note that the load-balancing is done by means of an `Nginx` app on front of the LLM node.

### Endpoints

- **GET /pokt/tokenizer** : Returns the full tokenizer files in JSON format: `special_tokens_map`, `tokenizer_config` and `tokenizer`.
- **GET /pokt/tokenizer-hash** : Returns the hash of the served tokenizer, it is useful if we have already interacted with this node.


### Setting-Up

Create a `.env` file with the following data:
```
SIDECAR_CONFIG_FILE=/<PATH TO>/config.json
SIDECAR_TOKENIZER_FILE=/<PATH TO TOKENIZER JSONS>/
CONFIG_PATH=/home/app/configs/config.json
```

build and deploy the docker image:
```bash
./build.sh
```



