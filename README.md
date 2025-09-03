# Kompox

## Introduction

Kompox is a Kubernetes-based, low-cost PaaS to host container web apps (e.g., Redmine, Gitea) with a single-replica, data-first approach.

This repository contains the Kompox CLI for infrastructure deployment and operations, called "kompoxops".

- Multi-tenant app hosting with strong data isolation
- Traefik-based ingress with DNS host-based routing
- Cloud-native volume (disk) and snapshot management decoupled from cluster lifecycle
- First target: Azure AKS (also planned: K3s and other managed Kubernetes)

## Documentation

You can find more documentation in the [docs directory](docs), mainly in Japanese.
