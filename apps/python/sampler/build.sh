imageNAME="sampler"
imageTAG=$(jq '. | .version' package.json | tr -d "\042")
echo "Building $imageNAME:$imageTAG"


# build image
docker build . -f Dockerfile --progress=plain -t "$imageNAME":"$imageTAG" -t "$imageNAME":latest
