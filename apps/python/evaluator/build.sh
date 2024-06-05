DEFAULT_IMAGE_NAME="pocket_ml_testbench_evaluator"

# go to root directory
cd ../../..

# Build sampler
docker build . -f apps/python/evaluator/Dockerfile --progress=plain --tag $DEFAULT_IMAGE_NAME:dev
# Broadcast image name and tag
echo "$DEFAULT_IMAGE_NAME"