DEFAULT_IMAGE_NAME="pocket_sampler"

# go to root directory
cd ../../..

# Build sampler
docker build . -f apps/python/sampler/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:latest
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"