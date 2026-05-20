#!/usr/bin/env bash

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

function waitForDeployment() {
  ns="$1"
  label="$2"
  print "waitForDeployment: $ns/$label"
  sleep 6
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s
  do
    sleep 4
    print "logs:"
    kubectl -n "$ns" logs -l "$label" || echo "could not get logs for $label"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l "$label"
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
  # Loop so the forward auto-restarts whenever the target pod is replaced (e.g. after helm upgrade).
  while kubectl cluster-info &>/dev/null 2>&1; do kubectl -n "$ns" port-forward "$deployment" "$ports" 2>/dev/null || true; sleep 2; done &
  print "portForwardAndWait: waiting until the port forward works..."
  sleep 1
  until nc -vz localhost "$portHere"
  do
    sleep 3
    print "logs:"
    kubectl -n "$ns" logs "$deployment"
    print "describe deployment:"
    kubectl -n "$ns" describe "$deployment"
    appName="${deployment##*/}"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l "app=${appName}" || echo "could not describe pod"
    print ...
  done
}
