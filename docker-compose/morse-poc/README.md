# Morse Proof of Concept - "One-Command" Environment

To execute the Pocket MLTB with one command, you first need to ensure you have the minimum hardware:
- RAM : `32 GB`
- CPU : Something rather fast? we tested on a `Ryzen 5950X` and a `MacBook Pro 2,3 GHz 8-Core Intel Core i9`.
- Disk : `~50 GB`, we need to download some heavy docker images and datasets.
- GPU : Anything from NVIDIA with at least `12 GB` VRAM. We tested on a `RTX3060`, `RTX3080` and `RTX4070`.

Then ensure that you have the software dependencies:
- [Docker Compose](https://docs.docker.com/compose/install/linux/)
- [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)
- [NVIDIA Drivers](https://www.nvidia.com/Download/index.aspx?lang=en-us)

Prepare the environment variables file (this command does not count!):
```bash
cp .env.sample .env
```

Edit the `.env`:
- `MODELS_PATH  = /where/you/want` : Just add a folder that will hold the models, the models are heavy
- `MODEL_NAME = casperhansen/llama-3-8b-instruct-awq` : This models gives you nice metrics.

Run the full PoC:
```bash
docker compose up -d
```

# Changing the Model - I'm GPU poor

(don't worry, we all are)

If you wish to change the model to an smaller one, you can change the `.env` file, modifying the `MODEL_NAME` the model name to another. For example a **really** small one is `facebook/opt-125m`.

You can also host the model on other machines if you want (we wont guide you through cloud deployment). To do so just change the configuration file in `pocket_configs/config/chains.json` to the following:
```json
[
  {
    "id": "A100",
    "url": "http://SERVICE_IP:SERVICE_PORT"
  }
]
```
where `SERVICE_IP` is the IP of the host and `SERVICE_PORT` is the port of the LLM service. Changing this file will create a Morse local-net whose nodes point to the provided service url.
Then, deploy all services but exclude the `llm-engine`:
```bash
docker compose up --scale llm-engine=0 -d
```