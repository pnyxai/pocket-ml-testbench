DEFAULT_IMAGE_NAME="pocket_ml_sidecar"

# go to root directory
cd ../../..

# Build sidecar
docker build . -f apps/python/sidecar/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:dev
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"