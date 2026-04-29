#!/usr/bin/env bash

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

function waitForDeployment() {
  ns="$1"
  label="$2"
  print "waitForDeployment: $ns/$label"
  sleep 10
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s
  do
    sleep 4
    print "logs:"
    kubectl -n "$ns" logs -l "$label" || echo "could not get logs for $label"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l "$label"
#    print "describe pod:"
#    kubectl -n "$ns" describe pod -l app=kuberpult-cd-service || echo "could not describe pod"
    print ...
  done
}

function portForwardAndWait() {
  ns="$1"
  deployment="$2"
  portHere="$3"
  portThere="$4"
  ports="$portHere:$portThere"
  print "portForwardAndWait for $ns/$deployment $ports"
  kubectl -n "$ns" port-forward "$deployment" "$ports" &
  print "portForwardAndWait: waiting until the port forward works..."
  sleep 10
  until nc -vz localhost "$portHere"
  do
    sleep 3
    print "logs:"
    kubectl -n "$ns" logs "$deployment"
    print "describe deployment:"
    kubectl -n "$ns" describe "$deployment"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l app=kuberpult-cd-service || echo "could not describe pod"
    print ...
  done
}
