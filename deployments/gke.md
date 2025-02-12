# Deploying Captain on Google Kubernetes Engine (GKE)

> [!WARNING]  
> This is an early version of the deployment process. Expect bugs and incomplete features. Use in production environments at your own risk.

## Prerequisites

- Google Cloud Platform account with billing enabled
- `gcloud` CLI installed and configured
- `kubectl` installed
- Docker installed (for building the image)

## Step 1: Create a GKE Cluster

```bash
# Create a GKE cluster with at least 3 nodes
gcloud container clusters create codecflow-cluster \
  --zone us-central1-a \
  --num-nodes 3 \
  --machine-type e2-standard-4 \
  --enable-autoscaling \
  --min-nodes 3 \
  --max-nodes 10
```

## Step 2: Configure kubectl

```bash
# Configure kubectl to use the GKE cluster
gcloud container clusters get-credentials codecflow-cluster --zone us-central1-a
```

## Step 3: Build and Push the Docker Image

```bash
# Build the Docker image
docker build -t gcr.io/YOUR_PROJECT_ID/captain:latest .

# Push the image to Google Container Registry
docker push gcr.io/YOUR_PROJECT_ID/captain:latest
```

## Step 4: Create Kubernetes Resources

### Create Namespace

```bash
kubectl create namespace codecflow
```

### Create RBAC Resources

```bash
kubectl apply -f rbac.yaml -n codecflow
```

### Create PVC for Entrypoint

```bash
cat <<EOF | kubectl apply -f - -n codecflow
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: entrypoint
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
```

### Deploy Captain

Update the `deployment.yaml` file to use your image:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: captain
spec:
  replicas: 1
  selector:
    matchLabels:
      app: captain
  template:
    metadata:
      labels:
        app: captain
    spec:
      serviceAccountName: captain-sa
      containers:
      - name: captain
        image: gcr.io/YOUR_PROJECT_ID/captain:latest
        ports:
        - containerPort: 9000
        env:
        - name: NAMESPACE
          value: "codecflow"
        - name: PREFIX
          value: "machine-"
        - name: ENTRYPOINT
          value: "entrypoint"
        - name: IMAGE
          value: "ghcr.io/codecflow/conductor:1.0.0"
```

Apply the deployment:

```bash
kubectl apply -f deployment.yaml -n codecflow
```

### Create Service

```bash
kubectl apply -f service.yaml -n codecflow
```

## Step 5: Expose the Service

### Option 1: Using LoadBalancer

```bash
kubectl patch svc captain -n codecflow -p '{"spec": {"type": "LoadBalancer"}}'
```

### Option 2: Using Ingress with Traefik

Apply the Traefik values:

```bash
kubectl apply -f traefik/values.yaml -n codecflow
```

Apply the IngressRoute:

```bash
kubectl apply -f traefik/ingressroute.yaml -n codecflow
```

## Step 6: Verify Deployment

```bash
# Check if pods are running
kubectl get pods -n codecflow

# Check the service
kubectl get svc -n codecflow

# Get the external IP (if using LoadBalancer)
kubectl get svc captain -n codecflow
```

## Known Issues

1. **Resource Limits**: The default resource limits may need adjustment based on your workload.
2. **Authentication**: The default API key authentication is not suitable for production. Implement a proper authentication system.
3. **Persistent Storage**: For production, consider using a more robust storage solution.
4. **Monitoring**: Add Prometheus monitoring for better observability.
5. **High Availability**: The current setup is not highly available. Consider running multiple replicas.

## Troubleshooting

### Pods Not Starting

Check the pod logs:

```bash
kubectl logs -f deployment/captain -n codecflow
```

### Permission Issues

Verify RBAC configuration:

```bash
kubectl describe role captain-role -n codecflow
kubectl describe rolebinding captain-rolebinding -n codecflow
```

### Network Issues

Check if the service is properly exposed:

```bash
kubectl describe svc captain -n codecflow
```

## Next Steps

- Set up monitoring with Prometheus and Grafana
- Configure proper authentication
- Implement backup and disaster recovery
- Set up CI/CD pipeline for automated deployments
