# Istio + Alloy tracing stack

This directory contains Kubernetes manifests for a local/demo setup with:

- Istio configured with an OpenTelemetry tracing provider that points to Alloy
- mesh-wide tracing enabled through the Istio Telemetry API
- an `observability` namespace with Alloy, Tempo, and Grafana
- a `demo` namespace with sidecar injection enabled

## Files

- `istio-install.yaml`: IstioOperator with `meshConfig.extensionProviders`
- `istio-telemetry.yaml`: mesh-wide Telemetry policy with 100% sampling
- `namespaces.yaml`: `observability` and `demo` namespaces
- `observability.yaml`: Tempo, Alloy, Grafana, and their config maps/services

## Apply order

```bash
kubectl apply -f deploy/istio-alloy/namespaces.yaml
istioctl install -y -f deploy/istio-alloy/istio-install.yaml
kubectl apply -f deploy/istio-alloy/istio-telemetry.yaml
kubectl apply -f deploy/istio-alloy/observability.yaml
kubectl get pods -n observability
```

## Port-forward

```bash
kubectl port-forward -n observability svc/grafana 3000:3000
kubectl port-forward -n observability svc/tempo 3200:3200
```

Grafana default credentials:

```text
admin / admin
```

If you later want external access through a LoadBalancer in Minikube, use
`minikube tunnel`.
