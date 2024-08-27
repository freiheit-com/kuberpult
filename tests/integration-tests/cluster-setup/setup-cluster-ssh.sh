#!/usr/bin/env bash

set -eu
set -o pipefail


scratch=$(mktemp -d)

ssh-keygen -t ed25519 -N "" -C host -f "${scratch}/host" &>/dev/null
ssh-keygen -t ed25519 -N "" -C client -f "${scratch}/client" &>/dev/null

host_pub="$(cat "${scratch}/host.pub")"

cp "${scratch}/client" ./client
cat <<EOF > known_hosts
server.${GIT_NAMESPACE}.svc.cluster.local ${host_pub}
localhost ${host_pub}
EOF

kubectl create namespace "git" || echo "already exists"
kubectl create namespace "argocd" || echo "already exists"

kubectl apply -f - <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: my-private-ssh-repo
  labels:
    argocd.argoproj.io/secret-type: repository
  namespace: ${ARGO_NAMESPACE}
stringData:
  project: test-env
  insecure: "true"
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
        app: git-server
    spec:
      initContainers:
      - image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1"
        imagePullPolicy: IfNotPresent
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
        imagePullPolicy: IfNotPresent
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
