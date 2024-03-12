# Morse Local Network - Proof of Concept

This file describes how to deploy a Pocket Network local net, staking ML models and perform inference.

# Setting up the Models

To run this proof of concept you will need at least one ML model available. You can do that by getting a compatible endpoint or by deplonying them locally.

- Large Language Models: Please see the provided [readme file](../model-deployment/llm/README.md). The endpoint to this service will be at `http://localhost:8000`
- Diffusers Models: Please see the provided [readme file](../model-deployment/diffusers/README.md). The endpoint to this service will be at `http://localhost:8001`


# Setting up the Pocket Local Net

In order to deploy the local net we will be using the [poktscan pocket-localnet repository](https://github.com/pokt-scan/pocket-localnet) and the [poktscan pokt-core repository](https://github.com/pokt-scan/pocket-core). We will modify the genesis configuration and deploy the ML chains.

1. Clone the localnet repo `cd .. && git clone https://github.com/pokt-scan/pocket-localnet.git`
2. Return to this repository folder `cd  pocket-ml-testbench`
3. Copy the modified genesis and chains files into the localnet repository 
    - `cp morse-localnet-poc/chains.json ../pocket-localnet/config/`
    - `cp morse-localnet-poc/genesis.json ../pocket-localnet/config/`
4. Perform the following steps detailed in the `pocket-localnet` readme (located at `../pocket-localnet/README.md`):
    - **Building** (except step 5, do not modify the `chains.json` file)
    - **Optional - Local Relayer**
5. Now you can start the localnet by doing `docker compose up lean1 lean2 lean3 mesh` inside the `pocket-localnet` repository folder. This will create the localnet and the blockchain will start to produce blocks. You can will be able to see block progression in [this local link](http://127.0.0.1:26647/status).

# Processing Relays

### Command line
asd

### Notebook


