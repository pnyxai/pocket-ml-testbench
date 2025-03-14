DEFAULT_IMAGE_NAME="pocket_ml_testbench_base_python_image"

# go to root directory
cd ../../..

# Build
docker build . -f apps/python/base_image/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:latest
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"