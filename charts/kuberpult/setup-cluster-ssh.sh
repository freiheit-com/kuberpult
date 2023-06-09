#!/usr/bin/env bash

set -eu
set -o pipefail


cd "$(dirname "$0")"

ARGO_NAMESPACE=default
#GIT_NAMESPACE=git # required to be set outside

scratch=$(mktemp -d)
function finish {
  rm -rf "$scratch"
}
trap finish EXIT

ssh-keygen -t ed25519 -N "" -C host -f "${scratch}/host" 1>&2
ssh-keygen -t ed25519 -N "" -C client -f "${scratch}/client" 1>&2

host_pub="$(cat "${scratch}/host.pub")"

cp "${scratch}/client" ../../services/cd-service/client
cat <<EOF > ../../services/cd-service/known_hosts
server.${GIT_NAMESPACE}.svc.cluster.local ${host_pub}
localhost ${host_pub}
EOF

echo printing known_hosts
cat ../../services/cd-service/known_hosts

kubectl create namespace "git" || echo "already exists"
kubectl create namespace "argocd" || echo "already exists"


docker pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1

kind load docker-image europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1

kubectl apply -f - <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: my-private-ssh-repo
  namespace: default
  labels:
    argocd.argoproj.io/secret-type: repository
  namespace: ${ARGO_NAMESPACE}
stringData:
  url: ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests
  sshPrivateKey: |
$(sed -e "s/^/    /" <"$scratch"/client)
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ssh-host
  namespace: ${GIT_NAMESPACE}
data:
  ssh_host_ed25519_key: |
$(sed -e "s/^/    /" <"$scratch"/host)
  ssh_host_ed25519_key.pub: |
$(sed -e "s/^/    /" <"$scratch"/host.pub)
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ssh-client
  namespace: ${GIT_NAMESPACE}
data:
  client.pub: |
$(sed -e "s/^/    /" <"$scratch"/client.pub)
---
apiVersion: v1
kind: Service
metadata:
  name: server
  namespace: ${GIT_NAMESPACE}
spec:
  ports:
  - name: ssh
    port: 22
    protocol: TCP
    targetPort: 22
  selector:
    app.kubernetes.io/name: server
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: server
  namespace: ${GIT_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: server
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app.kubernetes.io/name: server
    spec:
      initContainers:
      - image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1"
        imagePullPolicy: Never
        name: "git-init"
        command: ["/bin/sh","-c"]
        args: ["ls -l /template/; git init --bare /git/repos/manifests"]
        volumeMounts:
        - mountPath: /git/repos
          name: repos
        - name: template
          mountPath: /template
      containers:
      - image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1"
        imagePullPolicy: Never
        name: git
        ports:
        - containerPort: 22
          protocol: TCP
        env:
        - name: PUID
          value: "1000"
        - name: PGID
          value: "1000"
        volumeMounts:
        - mountPath: /git/keys-host
          name: ssh-host
          readOnly: true
        - mountPath: /git/keys
          name: ssh-client
          readOnly: true
        - mountPath: /git/repos
          name: repos
      volumes:
      - name: template # for initial test data
        hostPath:
          path: /create-testdata
      - name: ssh-host
        configMap:
          name: ssh-host
          defaultMode: 0600
      - name: ssh-client
        configMap:
          name: ssh-client
      - name: repos
        emptyDir:
          sizeLimit: 50Mi
      restartPolicy: Always
EOF
echo "done setting up ssh"
