#!/bin/bash

echo "=== Kubernetes Deployment Test ==="

# 1. Check if K8s is reachable
if ! kubectl cluster-info > /dev/null 2>&1; then
    echo "ERROR: Kubernetes cluster is not reachable. Please ensure 'Enable Kubernetes' is checked in Docker Desktop settings."
    exit 1
fi

# 2. Deploy Infrastructure Dependencies
echo "--- Deploying Infrastructure (Redis, Kafka, Milvus) ---"
kubectl apply -f deploy/kubernetes/test-infra.yaml

# 3. Build images
echo "--- Building Application Images ---"
docker build -t goverify-engine-api:latest -f deploy/Dockerfile.api .
docker build -t goverify-engine-worker:latest -f deploy/Dockerfile.worker .

# 4. Deploy Application using Helm
echo "--- Deploying Application using Helm ---"
helm upgrade --install goverify deploy/helm/goverify --set image.pullPolicy=Never

# 5. Wait for infrastructure to be ready first
echo "--- Waiting for Infrastructure to be ready ---"
kubectl wait --for=condition=ready pod -l app=milvus --timeout=120s

# 6. Wait for App pods to be ready
echo "--- Waiting for Application Pods to be ready ---"
kubectl wait --for=condition=ready pod -l app=kyc-api --timeout=60s
kubectl wait --for=condition=ready pod -l app=kyc-worker --timeout=60s

echo "--- Deployment Successful ---"
kubectl get pods

echo -e "\nTo test the API, run the following in a separate terminal:"
echo "kubectl port-forward svc/kyc-api 8080:8080"
echo -e "\nThen you can run: bash test_images.sh"
