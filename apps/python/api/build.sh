DEFAULT_IMAGE_NAME="pocket_ml_api"

# go to root directory
cd ../../..

# Build sidecar
docker build . -f apps/python/api/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:dev
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"