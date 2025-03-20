DEFAULT_IMAGE_NAME="pocket_ml_testbench_summarizer"

# go to root directory
cd ../../..

# Build base image
cd apps/python/base_image
bash build.sh
cd ../../..

# Build sampler
docker build . -f apps/python/summarizer/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:dev
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"