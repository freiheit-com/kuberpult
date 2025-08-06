#!/bin/sh

set -e;

#echo Waiting for K3s cluster to be ready;
#sleep 10 && kubectl wait --for=condition=Ready nodes --all --timeout=300s && sleep 3;

echo sleeping...
sleep 360

./tests/integration-tests/cluster-setup/setup-cluster-ssh.sh
./tests/integration-tests/cluster-setup/setup-postgres.sh
./tests/integration-tests/cluster-setup/argocd-kuberpult.sh
cd tests/integration-tests && go test "$GO_TEST_ARGS" ./...
./validation-check.sh || ./cluster-setup/get-logs.sh
echo ============ SUCCESS ============
