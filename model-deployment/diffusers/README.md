# Deploying Diffusser Models Locally

This document will describe how to deploy a Diffuser Model locally using [Diffusers API](https://github.com/RawthiL/diffusers_api), a simple wrapper over [Huggingface Diffusers](https://huggingface.co/docs/diffusers/index). Note that this API was created to mimic [Stable Diffusion](https://stablediffusionapi.com/docs/stable-diffusion-api) behavior, its not 100% compatible and is a work in progress.

### Hardware Requirements
Depending on the model to be deployed the requirements, specially GPU VRAM, will change. 

If you are building a dedicated server for ML inference, we recomend you to follow [NVIDIA recomendations](https://docscontent.nvidia.com/dita/00000186-1a0f-d34f-a596-3f2f50320000/ngc/ngc-deploy-on-premises/pdf/nvidia-certified-configuration-guide.pdf).

As an example, we will run a stable diffusion v1.4, specifically the [CompVis/stable-diffusion-v1-4](https://huggingface.co/CompVis/stable-diffusion-v1-4) variant. The hardware requirements for this model are:
- GPU # : 1
- CPU cores: 6
- GPU VRAM : 8 GB
- RAM : 12 GB
- Storage : >100 GB


### Set-Up

To run this model you will need to install the following:
1. Follow the [NVIDIA guide](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/index.html) to install NVIDIA drivers and CUDA > 12.1
2. Install [Docker](https://docs.docker.com/engine/install/) and the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).
3. Install [Git Large File Storage](https://git-lfs.com/)

### Model Download
1. Go to your download folder, we recommend it to be in a fast file system. `cd /foo/var`
2. Clone the selected model: `git clone https://huggingface.co/CompVis/stable-diffusion-v1-4`
    * The cloning can fail since the large files are really large, in that case just cancel the clonning and donwload the `model.safetensors` files manually using `wget` in the repository folder.


### Model Deployment

In this folder we provide you with a config file, `config.yaml`. You should change the `name` parameter under the section `model` to the actual folder where the model is located, specifically the path pointing to the `model_index.json` file.

To launch the model inference do:
```bash
docker run \         
    --gpus '"device=0"' \
    -v /foo/var:/models \
    -v /path_to_this_repo/model-deployment/diffusers/config.yaml:/config/config.yaml \
    -p 8001:80 \
    -it diffusers_api:latest 
```
Note that the paths `/foo/var` and `/path_to_this_repo` must be modified to where you cloned the model and where you cloned this repository, respectivelly.

After a while you should be able to see the start of the endpoint server:
```bash
INFO:     Started server process [1]
INFO:     Waiting for application startup.
INFO:     Application startup complete.
INFO:     Uvicorn running on http://0.0.0.0:80 (Press CTRL+C to quit)
```

### Model Testing

The API has three endpoints: `text2img`, `img2img` and `inpainting`. Since the later two require an image as input, we will provide only a test call for the first one:

```bash
curl http://localhost:8001/text2img \
    -H "Content-Type: application/json" \
    -d '{
        "prompt" : ["a person wearing a jacket with lots of pockets"],
        "negative_prompt" : [""],
        "height" : 512,
        "width" : 512,
        "num_inference_steps" : 25,
        "guidance_scale":  7.0,
        "output_type" : "PNG",
        "sag_scale" : 1.0 
    }'
```

The result will proabably spam your terminal, thats good, because thats the resulting image coded as PNG.
To find more information on how these endpoints work, see [this notebook](https://github.com/RawthiL/diffusers_api/blob/main/notebooks/API_test.ipynb).