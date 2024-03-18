DEFAULT_IMAGE_NAME="pocket_dataset_register"

# Build register
docker build . --progress=plain --tag $DEFAULT_IMAGE_NAME:latest
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"