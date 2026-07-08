# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A minimal Go HTTP server used as a study/learning project for Docker and Kubernetes (Full Cycle course). The server listens on `:8080` and returns a simple HTML response.

## Commands

```bash
# Run locally
go run server.go

# Build binary
go build -o server .

# Build Docker image
docker build -t emersondp07/hello-go .

# Push to Docker Hub
docker push emersondp07/hello-go:latest

# Create kind cluster (requires kind installed)
kind create cluster --config k8s/kind.yaml

# Load image into kind cluster (avoids pulling from Docker Hub)
kind load docker-image emersondp07/hello-go:latest

# Deploy pod to Kubernetes
kubectl apply -f k8s/pod.yaml

# Check pod status
kubectl get pods
```

## Architecture

- [server.go](server.go) — single-file Go HTTP server, no dependencies beyond stdlib
- [Dockerfile](Dockerfile) — builds with `golang:1.15`, copies sources and compiles
- [k8s/kind.yaml](k8s/kind.yaml) — kind cluster definition: 1 control-plane + 3 workers
- [k8s/pod.yaml](k8s/pod.yaml) — bare Pod manifest using image `emersondp07/hello-go:latest`

The Docker image must be pushed to Docker Hub (or loaded into kind) before the pod can start, since `pod.yaml` references the remote image tag.

## RBAC / Security

- [config/namespaces/security.yaml](config/namespaces/security.yaml) — `ServiceAccount` `server` + namespaced `Role`/`RoleBinding` (`server-read`), granting read-only access (`get`, `list`, `watch`) on `pods`, `services`, and `deployments` within the `dev` namespace only.
- [config/security-cluster.yaml](config/security-cluster.yaml) — same `ServiceAccount` `server` and same permission set, but as a `ClusterRole`/`ClusterRoleBinding`, granting read-only access cluster-wide (across all namespaces) instead of just `dev`.
- [config/namespaces/deployment.yaml](config/namespaces/deployment.yaml) sets `serviceAccountName: server` (instead of the `default` ServiceAccount) so the pod runs with these read-only permissions.
- Only apply one of the two (namespaced `Role` or cluster-wide `ClusterRole`) depending on whether the pod needs access limited to `dev` or across the whole cluster — applying both is redundant since the `ClusterRoleBinding` already covers everything the `RoleBinding` does.
