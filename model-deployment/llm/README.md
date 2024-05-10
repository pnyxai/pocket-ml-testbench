# Deploying Large Language Models Locally

This document will describe how to deploy a Large Language Model locally using [vLLM](https://github.com/vllm-project/vllm). This engine is fast, open source and can process [OpenAI](https://platform.openai.com/) API requests.

### Hardware Requirements
Depending on the model to be deployed the requirements, specially GPU VRAM, will change. To have a simple rule of VRAM requirements you can use the [following formula](https://www.substratus.ai/blog/calculating-gpu-memory-for-llm):

`VRAM = ( (<billon params> * 4bytes)/(32/<type bits (32, 16, 8, 4)>) ) * 1.2 `

If you are building a dedicated server for ML inference, we recomend you to follow [NVIDIA recomendations](https://docscontent.nvidia.com/dita/00000186-1a0f-d34f-a596-3f2f50320000/ngc/ngc-deploy-on-premises/pdf/nvidia-certified-configuration-guide.pdf).

As an example, we will run a Mistral 7B model with 4 Bits quantization, specifically the [NeuralHermes-2.5-Mistral-7B-AWQ](https://huggingface.co/TheBloke/NeuralHermes-2.5-Mistral-7B-AWQ) variant. The hardware requirements for this model are:
- GPU # : 1
- CPU cores: 6
- GPU VRAM : 8 GB
- RAM : 12 GB
- Storage : >100 GB

### Set-Up

To run this model you will need to install the following:
1. Follow the [NVIDIA guide](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/index.html) to install NVIDIA drivers and CUDA > 12.1
2. Install [Docker](https://docs.docker.com/engine/install/), [Docker Compose](https://docs.docker.com/compose/) and the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).
3. Install [Git Large File Storage](https://git-lfs.com/)

### Model Download
1. Go to your download folder, we recommend it to be in a fast file system. `cd /foo/var`
2. Clone the selected model: `git clone https://huggingface.co/TheBloke/NeuralHermes-2.5-Mistral-7B-AWQ`
    * The cloning can fail since the large files are really large, in that case just cancel the clonning and donwload the `model.safetensors` file manually using `wget` in the repository folder.

### Model Deployment

In this folder we provide you with three files:
- `docker-compose.yml`
- `.env`

The `docker-compose.yml` file needs no edits, except if you are using a multi-GPU system and you want to specify the GPU. By default we select the `GPU 0` using PCIe order.
The `.env` file contains the specifications of the model to use and how to use the system resources. There are several fields that need to be edit:
- `MODELS_PATH` Set this variable to a path in the host file system. That path is the one where you downloaded the model (or the path to your custom model).
- `MODEL_NAME` Should not be change if you use the `NeuralHermes-2.5-Mistral-7B-AWQ` model. If you wish to change the model, please just change the last part of the path. The first part, `/root/.cache/huggingface/hub/`, is the cache location inside the docker image, do not modify that.
- `SERVED_MODEL_NAME` you can set this to any name, you dont need to change this to run the example code. You will be using this string to route requests to the model.

Once you edited the files to reflect your needs, you just need to run the compose:
```bash
docker compose up                                                                            
```

After a while you should be able to see the start of the endpoint server:
```bash
vllm-openai-main  | INFO:     Started server process [1]
vllm-openai-main  | INFO:     Waiting for application startup.
vllm-openai-main  | INFO:     Application startup complete.
vllm-openai-main  | INFO:     Uvicorn running on http://0.0.0.0:8000 (Press CTRL+C to quit)
```

### Model Testing

The endpoints of vLLM try to follow OpenAI format (as any model staked in the Pocket Network should). 
To test the `completitions` endpoint just do:
```bash
curl http://localhost:8000/v1/completions \
    -H "Content-Type: application/json" \
    -d '{
        "model": "pocket_network",
        "prompt": "Write a short description of the Pocket Network.",
        "max_tokens": 2048,
        "temperature": 0.0
    }'
```
And the `chat` endpoint using:
```bash
curl http://localhost:8000/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{
        "model": "pocket_network",
        "messages": [{"role": "user", "content": "I am PoktNews"},
{"role": "assistant", "content": "Hello, PoktNews!"},
{"role": "user", "content": "What is my name?"},
{"role": "assistant", "content": "Nobody knows."},
{"role": "user", "content": "What is your best guess?"}],
        "max_tokens": 2048,
        "temperature": 0.5
    }'
```

The answers to this queries will probably be allucinations, but the important part is to get something like these answers:

```json
{"id":"cmpl-b2e43e2827f14bf9b409f842b4e448de","object":"text_completion","created":143824,"model":"pocket_network","choices":[{"index":0,"text":"\n\nPocket Network is a decentralized infrastructure for Web3 applications that enables secure and reliable communication between them. It is built on top of the InterPlanetary File System (IPFS) and utilizes a network of nodes to facilitate communication between different applications.\n\nWhat are the benefits of using Pocket Network?\n\nThe benefits of using Pocket Network include:\n\n1. Decentralization: Pocket Network is built on a decentralized infrastructure, which means that there is no single point of failure. This ensures that the network is resilient and can withstand any attacks or malicious activities.\n\n2. Security: Pocket Network uses a consensus mechanism to ensure that all communication between applications is secure and reliable. This means that data is encrypted and can only be accessed by authorized parties.\n\n3. Scalability: Pocket Network is designed to be scalable, which means that it can handle a large number of requests and transactions. This makes it ideal for use in high-traffic applications.\n\n4. Cost-effective: Pocket Network uses a token-based system to incentivize node operators to join the network. This means that users can access the network at a lower cost compared to traditional centralized infrastructure.\n\n5. Interoperability: Pocket Network is designed to work with a wide range of applications and protocols, making it easy to integrate into existing systems.\n\nWhat are the use cases of Pocket Network?\n\nPocket Network has a wide range of use cases, including:\n\n1. Decentralized applications (dApps): Pocket Network can be used to facilitate communication between different dApps, ensuring that data is secure and reliable.\n\n2. Blockchain-based games: Pocket Network can be used to enable communication between different blockchain-based games, allowing for seamless integration and data sharing.\n\n3. Decentralized exchanges (DEXs): Pocket Network can be used to facilitate communication between different DEXs, ensuring that trades are executed securely and reliably.\n\n4. Decentralized finance (DeFi) applications: Pocket Network can be used to enable communication between different DeFi applications, allowing for seamless integration and data sharing.\n\n5. Supply chain management: Pocket Network can be used to facilitate communication between different parties involved in supply chain management, ensuring that data is secure and reliable.","logprobs":null,"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"total_tokens":522,"completion_tokens":511}}
```

```json
{"id":"cmpl-c294156d0b8541c19471a9c9454c4141","object":"text_completion","created":143783,"model":"pocket_network","choices":[{"index":0,"text":"\n\nPocket Network is a decentralized infrastructure for Web3 that connects blockchain applications with APIs, data, and services. It is built on top of the Ethereum blockchain and utilizes the InterPlanetary File System (IPFS) for storing and accessing data. Pocket Network provides a secure and reliable way for blockchain applications to access external data and services, without relying on centralized servers.\n\nThe Pocket Network is made up of a network of nodes, which are run by volunteers who are incentivized to provide reliable and fast data access to blockchain applications. These nodes are connected through a decentralized consensus mechanism, which ensures that data is accurate and consistent across the network.\n\nThe Pocket Network also provides a marketplace for data and services, where providers can offer their products and services to blockchain applications, and consumers can purchase the data and services they need. This marketplace is built on top of a decentralized autonomous organization (DAO), which ensures fair and transparent operations.\n\nOverall, the Pocket Network provides a decentralized solution for blockchain applications to access external data and services, while ensuring security, reliability, and fairness.","logprobs":null,"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"total_tokens":266,"completion_tokens":255}}
```

### Stoping

To stop the inference server just do `docker compose down --volumes`.