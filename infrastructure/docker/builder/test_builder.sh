SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";
docker build -t kuberpult-builder .
docker run -d --privileged -v $SCRIPT_DIR/../../..:/repo kuberpult-builder
id=$(docker ps | grep "kuberpult-builder" | cut -f1 -d" ")
docker exec $id sh -c 'cd /repo/services/cd-service; make docker'
docker kill $id
docker rm $id

