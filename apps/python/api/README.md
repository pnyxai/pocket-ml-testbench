# Machine Learning Test Bench API

This is a proof of concept for an API (outside the scope of the [original socket](https://forum.pokt.network/t/open-pokt-ai-lab-socket/5056)).
It was created with the purpose of calculating the average scores shown in the [Open LLM Leaderboard](https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard) using the data collected by the MLTB and provide the table data to the [leaderboard site](../../nodejs/web/README.md).

### Endpoints

- **GET /leaderboard** : A simple endpoint that will return the data of the open llm leaderboard for every node in the network.


### Setting-Up

Create an environment variable with the URI of the Mongo Database, for example:

`MONGODB_URI=mongodb://mongodb:27017/pocket-ml-testbench`

build and deploy the docker image:
```bash
./build.sh
```



