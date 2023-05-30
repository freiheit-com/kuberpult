#!/usr/bin/env bash

ARGO_NAMESPACE=argocd
GIT_NAMESPACE=git

scratch=$(mktemp -d)
function finish {
  rm -rf "$scratch"
}
trap finish EXIT

ssh-keygen -t ed25519 -N "" -C host -f "${scratch}/host" 1>&2
ssh-keygen -t ed25519 -N "" -C client -f "${scratch}/client" 1>&2

host_pub="$(cat "${scratch}/host.pub")"

cp "${scratch}/client" ./services/cd-service/client
cat <<EOF > ./services/cd-service/known_hosts
127.0.0.1 ${host_pub}
EOF

cat <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: argocd-ssh-known-hosts-cm
    app.kubernetes.io/part-of: argocd
  name: argocd-ssh-known-hosts-cm
  namespace: ${ARGO_NAMESPACE}
data:
  ssh_known_hosts: |
    server.${GIT_NAMESPACE}.svc.cluster.local ${host_pub}
---
apiVersion: v1
kind: Secret
metadata:
  name: my-private-ssh-repo
  namespace: argocd
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
      - image: "git-ssh:latest"
        imagePullPolicy: Never
        name: "git-init"
        command:
        - git
        - init
        - "--bare"
        - "/git/repos/manifests"
        volumeMounts:
        - mountPath: /git/repos
          name: repos
      containers:
      - image: "git-ssh:latest"
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
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root
  namespace: ${ARGO_NAMESPACE}
spec:
  destination:
    namespace: ${ARGO_NAMESPACE}
    server: https://kubernetes.default.svc
  project: default
  source:
    path: ./argocd/v1alpha1
    repoURL: ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests
    targetRevision: HEAD
  syncPolicy:
    automated: {}
EOF
