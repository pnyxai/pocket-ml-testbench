DEFAULT_IMAGE_NAME="pocket_llm_register" # vllm.entrypoints.openai.api_server --> "/v1/chat/completions" & "/v1/completions"

# Build register
docker build . --progress=plain --tag $DEFAULT_IMAGE_NAME:latest
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"