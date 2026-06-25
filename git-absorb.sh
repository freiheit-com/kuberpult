#!/usr/bin/env bash
#
# git-absorb.sh — test "absorb the version bump INTO the bracket move" (Way A, version-safe).
#
# The keep-in-both move already renders the app in BOTH brackets during the hand-off window. The
# trick: render the NEW version in BOTH brackets. Then the two Argo apps still flap ownership, but
# on IDENTICAL manifests (both v_new), so there is NO version thrash — the workload rolls
# v_old -> v_new exactly ONCE and stays. This removes the "no version change during a move" rule:
# the real rule is just "both brackets must render the SAME version during the window."
#
#   move    (commit1): loser -> Prune=false; write app@NEW to BOTH loser AND winner brackets.
#                      app is now in both, both @ NEW. Owner flaps (harmless); single roll to NEW.
#   evict   (commit2): remove app from loser (loser already Prune=false -> no prune). Flap stops,
#                      winner becomes sole owner.
#   finalize(commit3): restore loser -> Prune=true (winner owns the app now, nothing to prune).
#
# Contrast: git3.sh leaves the loser at v_old (so a version bump would thrash). git-absorb.sh
# updates the loser to v_new too, so the bump rides along for free — no extra commit, no thrash.
#
# This script ONLY pushes to git; Argo drives the cluster (your app-of-apps applies the
# Applications under argocd/v1alpha1/). Only READ-ONLY kubectl for observation.
#
# ---- SIMPLE FLOW: move a1 from b1 -> b2, bumping v_old -> v_new --------------------------------
#   export REPO=~/path/to/your/argo-connected-repo   # required
#   ./git-absorb.sh observe        # second terminal, leave running
#   ./git-absorb.sh setup          # b1 = {a1@OLD, keeper@OLD}, prune ON; b2 absent
#   ./git-absorb.sh move           # commit1: b1 Prune=false; write a1@NEW to BOTH b1 and b2
#   #   wait until ./git-absorb.sh owner shows a1 flapping but image == NEW (no OLD<->NEW churn)
#   ./git-absorb.sh evict          # commit2: remove a1 from b1 (Prune=false -> no prune)
#   #   wait until owner settles on <env>-b2
#   ./git-absorb.sh finalize       # commit3: restore b1 Prune=true
#   ./git-absorb.sh status ; ./git-absorb.sh teardown
#
# ---- SWAP FLOW: a1 b1->b2 AND a2 b2->b1, both bumping versions --------------------------------
#   ./git-absorb.sh setup-swap     # b1 = {a1@OLD, keeper1}, b2 = {a2@OLD, keeper2}, both prune ON
#   ./git-absorb.sh move-swap      # b1&b2 Prune=false; a1@NEW into BOTH, a2@NEW into BOTH
#   ./git-absorb.sh evict-swap     # remove a1 from b1 and a2 from b2
#   ./git-absorb.sh finalize-swap  # restore b1 AND b2 to Prune=true
#   ./git-absorb.sh status ; ./git-absorb.sh teardown
#
# PASS criterion: each app's Deployment UID + creationTimestamp stay CONSTANT; the running image
# rolls OLD -> NEW exactly ONCE and never oscillates OLD<->NEW (that oscillation would mean the
# version thrash we're trying to avoid); owner may flap during move->evict, then settles.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config (override via env)
# ---------------------------------------------------------------------------
REPO="${REPO:-}"                       # local clone of your Argo-connected repo (REQUIRED)
BRANCH="${BRANCH:-main}"
NS="${NS:-tools}"                      # EXISTING namespace for the throwaway workloads
ENVNAME="${ENVNAME:-development}"      # the env dir under environments/
IMAGE_OLD="${IMAGE_OLD:-nginx:1.25-alpine}"   # version at setup
IMAGE_NEW="${IMAGE_NEW:-nginx:1.27-alpine}"   # version the move deploys into BOTH brackets

A1="${A1:-demo-a1}"                    # moves b1 -> b2 (both flows)
A2="${A2:-demo-a2}"                    # moves b2 -> b1 (swap flow only)
K1="${K1:-keeper-1}"                   # stays in b1
K2="${K2:-keeper-2}"                   # stays in b2 (swap flow only)

B1="b1"                                # bracket names
B2="b2"
APP_B1="${ENVNAME}-${B1}"              # Argo Application for bracket b1
APP_B2="${ENVNAME}-${B2}"             # Argo Application for bracket b2

B1_DIR="environments/${ENVNAME}/brackets/${B1}"
B2_DIR="environments/${ENVNAME}/brackets/${B2}"
ARGO_DIR="argocd/v1alpha1"
APP_B1_FILE="${ARGO_DIR}/${APP_B1}.yaml"
APP_B2_FILE="${ARGO_DIR}/${APP_B2}.yaml"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo ">>> $*"; }

require_repo() {
  [ -n "$REPO" ] || die "set REPO=/path/to/your/argo-connected-repo"
  [ -d "$REPO/.git" ] || die "REPO ($REPO) is not a git repo"
  REPO_URL="$(git -C "$REPO" remote get-url origin)"
  info "REPO=$REPO  REPO_URL=$REPO_URL  BRANCH=$BRANCH  (absorb ${IMAGE_OLD} -> ${IMAGE_NEW})"
}

git_commit_push() { # $1 = message
  git -C "$REPO" add -A
  git -C "$REPO" commit -m "$1"
  git -C "$REPO" push origin "$BRANCH"
}

clean_test_artifacts() { # clean slate: delete ALL brackets + their Argo Applications
  rm -rf "$REPO/environments/${ENVNAME}/brackets"
  rm -f  "$REPO/${ARGO_DIR}/${ENVNAME}-"*.yaml
}

emit_deploy() { # $1 = deployment/app name  $2 = image
  local name="$1" image="$2"
  cat <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
        - name: app
          image: ${image}
          ports:
            - containerPort: 80
YAML
}

emit_app() { # $1=app name  $2=bracket dir  $3=prune(on|off)
  local name="$1" path="$2" prune="$3"
  cat <<YAML
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ${name}
  namespace: tools
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: '*'
  source:
    repoURL: ${REPO_URL}
    targetRevision: ${BRANCH}
    path: ${path}
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: true
YAML
  if [ "$prune" = "off" ]; then
    # Suppress DELETION only (loser hand-off guard). Does NOT stop adoption/self-heal.
    cat <<YAML
    syncOptions:
      - Prune=false
YAML
  fi
}

# ---------------------------------------------------------------------------
# SIMPLE FLOW: a1 moves b1 -> b2 (absorbing the version bump)
# ---------------------------------------------------------------------------
step_setup() { # clean slate: b1 = {a1@OLD, keeper@OLD}, prune ON; b2 absent
  require_repo
  clean_test_artifacts
  mkdir -p "$REPO/$B1_DIR" "$REPO/$ARGO_DIR"
  emit_deploy "$A1" "$IMAGE_OLD" > "$REPO/$B1_DIR/${A1}.yaml"
  emit_deploy "$K1" "$IMAGE_OLD" > "$REPO/$B1_DIR/${K1}.yaml"
  emit_app "$APP_B1" "$B1_DIR" on > "$REPO/$APP_B1_FILE"
  git_commit_push "git-absorb simple: setup (b1 = {${A1}, ${K1}} @ ${IMAGE_OLD}, prune on)"
  info "expect: ${A1} and ${K1} created in ns ${NS} @ ${IMAGE_OLD}, both owned by ${APP_B1}. Note ${A1}'s UID."
}

step_move() { # commit1: loser Prune=false; write a1@NEW to BOTH b1 (update) and b2 (winner)
  require_repo
  mkdir -p "$REPO/$B2_DIR"
  emit_deploy "$A1" "$IMAGE_NEW" > "$REPO/$B1_DIR/${A1}.yaml"   # UPDATE loser copy to NEW
  emit_deploy "$A1" "$IMAGE_NEW" > "$REPO/$B2_DIR/${A1}.yaml"   # winner copy at NEW
  emit_app "$APP_B2" "$B2_DIR" on  > "$REPO/$APP_B2_FILE"        # winner: prune ON (adopts ${A1})
  emit_app "$APP_B1" "$B1_DIR" off > "$REPO/$APP_B1_FILE"        # loser: Prune=false, still renders ${A1}
  git_commit_push "git-absorb simple: move ${A1} -> ${B2} @ ${IMAGE_NEW} (written to BOTH; loser ${B1} Prune=false)"
  info "expect: ${A1} in BOTH brackets, BOTH at ${IMAGE_NEW}. Owner may flap, but image is ${IMAGE_NEW}"
  info "        in both -> single roll ${IMAGE_OLD} -> ${IMAGE_NEW}, NO OLD<->NEW oscillation. ${B1} won't prune."
  info "        WAIT until image is stable at ${IMAGE_NEW}, then './git-absorb.sh evict'."
}

step_evict() { # commit2: remove a1 from loser b1 (b1 already Prune=false)
  require_repo
  rm -f "$REPO/$B1_DIR/${A1}.yaml"
  emit_app "$APP_B1" "$B1_DIR" off > "$REPO/$APP_B1_FILE"        # keep Prune=false
  git_commit_push "git-absorb simple: evict ${A1} from ${B1} (${B1} stays Prune=false)"
  info "expect: ${B1} skips prune (Prune=false). ${A1} now only in ${B2} -> flap stops, ${APP_B2} sole owner."
  info "        WAIT until ./git-absorb.sh owner shows ${A1} on ${APP_B2} before 'finalize'."
}

step_finalize() { # commit3: restore loser b1 to Prune=true
  require_repo
  emit_app "$APP_B1" "$B1_DIR" on > "$REPO/$APP_B1_FILE"
  git_commit_push "git-absorb simple: finalize (${B1} Prune=true restored)"
  info "expect: ${A1} owned by ${APP_B2} @ ${IMAGE_NEW}; ${B1} has nothing to prune. Prune=true everywhere."
}

# ---------------------------------------------------------------------------
# SWAP FLOW: a1 b1->b2 AND a2 b2->b1 (both absorbing version bumps)
# ---------------------------------------------------------------------------
step_setup_swap() { # b1 = {a1@OLD, k1@OLD}, b2 = {a2@OLD, k2@OLD}, both prune ON
  require_repo
  clean_test_artifacts
  mkdir -p "$REPO/$B1_DIR" "$REPO/$B2_DIR" "$REPO/$ARGO_DIR"
  emit_deploy "$A1" "$IMAGE_OLD" > "$REPO/$B1_DIR/${A1}.yaml"
  emit_deploy "$K1" "$IMAGE_OLD" > "$REPO/$B1_DIR/${K1}.yaml"
  emit_deploy "$A2" "$IMAGE_OLD" > "$REPO/$B2_DIR/${A2}.yaml"
  emit_deploy "$K2" "$IMAGE_OLD" > "$REPO/$B2_DIR/${K2}.yaml"
  emit_app "$APP_B1" "$B1_DIR" on > "$REPO/$APP_B1_FILE"
  emit_app "$APP_B2" "$B2_DIR" on > "$REPO/$APP_B2_FILE"
  git_commit_push "git-absorb swap: setup (b1 = {${A1}, ${K1}}, b2 = {${A2}, ${K2}} @ ${IMAGE_OLD}, prune on)"
  info "expect: ${A1}+${K1} owned by ${APP_B1}; ${A2}+${K2} owned by ${APP_B2}, all @ ${IMAGE_OLD}. Note both UIDs."
}

step_move_swap() { # commit1: both Prune=false; write a1@NEW to BOTH, a2@NEW to BOTH
  require_repo
  emit_deploy "$A1" "$IMAGE_NEW" > "$REPO/$B1_DIR/${A1}.yaml"   # a1 update in loser b1
  emit_deploy "$A1" "$IMAGE_NEW" > "$REPO/$B2_DIR/${A1}.yaml"   # a1 into winner b2
  emit_deploy "$A2" "$IMAGE_NEW" > "$REPO/$B2_DIR/${A2}.yaml"   # a2 update in loser b2
  emit_deploy "$A2" "$IMAGE_NEW" > "$REPO/$B1_DIR/${A2}.yaml"   # a2 into winner b1
  emit_app "$APP_B1" "$B1_DIR" off > "$REPO/$APP_B1_FILE"        # both Prune=false
  emit_app "$APP_B2" "$B2_DIR" off > "$REPO/$APP_B2_FILE"
  git_commit_push "git-absorb swap: move (${A1}&${A2} @ ${IMAGE_NEW} into BOTH; ${B1}&${B2} Prune=false)"
  info "expect: ${A1} in both @ ${IMAGE_NEW}, ${A2} in both @ ${IMAGE_NEW}. Owners flap, images stable at ${IMAGE_NEW}."
  info "        WAIT until images stable at ${IMAGE_NEW} before 'evict-swap'."
}

step_evict_swap() { # commit2: remove a1 from b1 and a2 from b2 (both already Prune=false)
  require_repo
  rm -f "$REPO/$B1_DIR/${A1}.yaml"
  rm -f "$REPO/$B2_DIR/${A2}.yaml"
  emit_app "$APP_B1" "$B1_DIR" off > "$REPO/$APP_B1_FILE"        # keep Prune=false
  emit_app "$APP_B2" "$B2_DIR" off > "$REPO/$APP_B2_FILE"
  git_commit_push "git-absorb swap: evict (${A1} out of ${B1}, ${A2} out of ${B2}; both stay Prune=false)"
  info "expect: no prune (both Prune=false). ${A1}->${APP_B2}, ${A2}->${APP_B1} settle as sole owners."
}

step_finalize_swap() { # commit3: restore BOTH brackets to Prune=true
  require_repo
  emit_app "$APP_B1" "$B1_DIR" on > "$REPO/$APP_B1_FILE"
  emit_app "$APP_B2" "$B2_DIR" on > "$REPO/$APP_B2_FILE"
  git_commit_push "git-absorb swap: finalize (${B1} & ${B2} Prune=true restored)"
  info "expect: swap complete. ${A1} owned by ${APP_B2}, ${A2} owned by ${APP_B1}, both @ ${IMAGE_NEW}. Prune=true everywhere."
}

# ---------------------------------------------------------------------------
# Shared teardown
# ---------------------------------------------------------------------------
step_teardown() {
  require_repo
  clean_test_artifacts
  git_commit_push "git-absorb: teardown (remove all brackets + Applications)"
  info "pushed removal. If your app-of-apps does NOT auto-prune Applications, delete them manually:"
  info "  kubectl -n argocd delete app ${APP_B1} ${APP_B2}"
}

# ---------------------------------------------------------------------------
# Observation (read-only kubectl)
# ---------------------------------------------------------------------------
owner_of() { # $1 = deployment name
  kubectl -n "$NS" get deploy "$1" \
    -o jsonpath='{.metadata.labels.argocd\.argoproj\.io/instance}{"\n"}' 2>/dev/null || true
}

image_of() { # $1 = deployment name
  kubectl -n "$NS" get deploy "$1" \
    -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}' 2>/dev/null || true
}

step_owner() { # owner + image of a1 (and a2 if present); image must NOT oscillate OLD<->NEW
  for app in "$A1" "$A2"; do
    echo "${app}: owner=$(owner_of "$app")  image=$(image_of "$app")"
  done
}

step_status() { # owners + images + syncOptions
  for app in "$A1" "$A2"; do
    echo "=== ${app}: owner=$(owner_of "$app")  image=$(image_of "$app") ==="
  done
  for a in "$APP_B1" "$APP_B2"; do
    echo "=== ${a}: sync/health + syncOptions (Prune=false during the move; gone after finalize) ==="
    kubectl -n argocd get application "$a" \
      -o jsonpath='sync={.status.sync.status} health={.status.health.status} syncOptions={.spec.syncPolicy.syncOptions}{"\n"}' 2>/dev/null || true
  done
}

step_observe() { # second terminal; Ctrl-C to stop
  while true; do
    echo "---------------------------------------------------------------"
    for app in "$A1" "$A2"; do
      echo "=== ${app}: UID + CREATED constant | IMAGE rolls OLD->NEW once (no OLD<->NEW churn) | owner ==="
      kubectl -n "$NS" get deploy "$app" \
        -o custom-columns=UID:.metadata.uid,CREATED:.metadata.creationTimestamp,IMAGE:.spec.template.spec.containers[0].image 2>/dev/null || true
      echo "owner: $(owner_of "$app")"
      kubectl -n "$NS" get pods -l app="$app" -o wide 2>/dev/null || true
      echo
    done
    sleep 2
  done
}

usage() {
  cat <<EOF
usage: REPO=/path/to/repo ./git-absorb.sh <step>

Absorb the version bump INTO the move: write the NEW version to BOTH brackets during the
keep-in-both window -> identical manifests -> harmless owner flap, single clean roll to NEW.
versions: ${IMAGE_OLD} -> ${IMAGE_NEW}

simple flow (a1 moves ${B1} -> ${B2}, bumping version):
  setup          b1 = {${A1}, ${K1}} @ ${IMAGE_OLD}, prune on; b2 absent
  move           commit1 — ${B1} Prune=false; write ${A1}@${IMAGE_NEW} to BOTH ${B1} and ${B2}
  evict          commit2 — remove ${A1} from ${B1} (already Prune=false)
  finalize       commit3 — restore ${B1} Prune=true

swap flow (a1 ${B1}->${B2} AND a2 ${B2}->${B1}, both bumping versions):
  setup-swap     b1 = {${A1}, ${K1}}, b2 = {${A2}, ${K2}} @ ${IMAGE_OLD}, both prune on
  move-swap      commit1 — both Prune=false; ${A1}@${IMAGE_NEW} into BOTH, ${A2}@${IMAGE_NEW} into BOTH
  evict-swap     commit2 — remove ${A1} from ${B1} and ${A2} from ${B2}
  finalize-swap  commit3 — restore ${B1} AND ${B2} to Prune=true

shared:
  teardown       remove all brackets + Applications
  observe        watch ${A1}/${A2} (second terminal): UID, image, restarts, owner
  owner          print current owner + image (image must NOT oscillate ${IMAGE_OLD}<->${IMAGE_NEW})
  status         owners + images + syncOptions

config (env): REPO(required) BRANCH=$BRANCH NS=$NS ENVNAME=$ENVNAME IMAGE_OLD=$IMAGE_OLD IMAGE_NEW=$IMAGE_NEW A1=$A1 A2=$A2 K1=$K1 K2=$K2
EOF
}

case "${1:-}" in
  setup)         step_setup ;;
  move)          step_move ;;
  evict)         step_evict ;;
  finalize)      step_finalize ;;
  setup-swap)    step_setup_swap ;;
  move-swap)     step_move_swap ;;
  evict-swap)    step_evict_swap ;;
  finalize-swap) step_finalize_swap ;;
  teardown)      step_teardown ;;
  owner)         step_owner ;;
  status)        step_status ;;
  observe)       step_observe ;;
  *)             usage ;;
esac
