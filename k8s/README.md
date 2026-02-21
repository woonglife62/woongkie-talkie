# Kubernetes Deployment Guide

## Prerequisites

- kubectl configured to point at your cluster
- Docker image built and pushed to a registry accessible by the cluster

## Build & Push Docker Image

```bash
docker build -t your-registry/woongkie-talkie:latest .
docker push your-registry/woongkie-talkie:latest
```

Update the `image:` field in `deployment.yaml` to match your registry path.

## Configure Secrets

Edit `secret.yaml` and replace the placeholder base64 values with real ones:

```bash
# Encode each value
echo -n 'your-mongodb-username' | base64
echo -n 'your-mongodb-password' | base64
echo -n 'your-jwt-secret-minimum-32-characters' | base64
echo -n 'your-redis-password' | base64
```

Paste the output into the corresponding fields in `secret.yaml`.

## Configure Domain

Edit `ingress.yaml` and replace `your-domain.com` with your actual domain. Uncomment the `tls:` section if using HTTPS.

## Deploy

```bash
# Apply all manifests in order
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/hpa.yaml

# Or apply all at once (namespace must exist first)
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/
```

## Verify Deployment

```bash
# Check pods are running
kubectl get pods -n woongkie-talkie

# Check logs
kubectl logs -n woongkie-talkie -l app.kubernetes.io/name=woongkie-talkie

# Check HPA status
kubectl get hpa -n woongkie-talkie
```

## Architecture Notes

### No Sticky Sessions Required

This application uses **Redis Pub/Sub** for cross-server messaging. All WebSocket messages published on any pod are forwarded to every subscriber on every other pod. This means:

- Any pod can handle any user's WebSocket connection
- Standard round-robin load balancing works correctly
- No session affinity (`sessionAffinity: ClientIP`) is needed on the Service
- Horizontal scaling (via HPA) works without coordination overhead

### WebSocket Timeouts

The Ingress is configured with 3600-second (1 hour) proxy timeouts to support long-lived WebSocket connections. The nginx annotations set:

- `proxy-read-timeout: 3600`
- `proxy-send-timeout: 3600`
- `proxy-connect-timeout: 3600`

### Health Checks

- **Liveness** (`/health`): restarts the pod if the app is unhealthy
- **Readiness** (`/ready`): removes the pod from the load balancer if not ready to serve traffic
- **Startup probe**: gives the app up to 60s (12 attempts x 5s) to start before liveness kicks in

### Resource Limits

Each pod is capped at 256Mi memory and 250m CPU. The HPA scales between 2 and 10 replicas based on:
- CPU > 70% average utilization
- Memory > 80% average utilization
