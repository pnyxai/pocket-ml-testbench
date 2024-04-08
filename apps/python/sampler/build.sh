DEFAULT_IMAGE_NAME="pocket_sampler"

# Build register
docker build . --progress=plain --tag $DEFAULT_IMAGE_NAME:latest
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"