#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="podustrial"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker wurde nicht gefunden. Bitte installiere Docker Desktop: https://www.docker.com/products/docker-desktop/" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "Docker läuft nicht. Bitte starte Docker Desktop und versuche es erneut." >&2
  exit 1
fi

if ! command -v kind >/dev/null 2>&1; then
  echo "kind wurde nicht gefunden. Installationsanleitung: https://kind.sigs.k8s.io/docs/user/quick-start/#installation" >&2
  exit 1
fi

cleanup_on_failure() {
  echo "Cluster-Erstellung fehlgeschlagen, räume auf..." >&2
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
}

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  trap cleanup_on_failure ERR
  kind create cluster --name "$CLUSTER_NAME" --config kind-config.yaml
  trap - ERR
fi

export KUBECONFIG
KUBECONFIG="$(kind get kubeconfig-path --name "$CLUSTER_NAME" 2>/dev/null || true)"
kind export kubeconfig --name "$CLUSTER_NAME"

go run ./cmd/server
