# GoVerify Engine - Professional Makefile

# Configuration
HELM_RELEASE_NAME=goverify
HELM_CHART_PATH=deploy/helm/goverify
NAMESPACE=default

# Colors for output
BLUE=\033[0;34m
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: all build deploy status logs clean help test

help:
	@echo "$(BLUE)GoVerify Engine Management$(NC)"
	@echo "Usage:"
	@echo "  make build    - Build all local Docker images"
	@echo "  make deploy   - Install/Upgrade Helm chart and wait for pods"
	@echo "  make status   - Check system health and pod status"
	@echo "  make logs     - Tail logs of the KYC API"
	@echo "  make test     - Run end-to-end AI service tests"
	@echo "  make clean    - Uninstall Helm release"

all: build deploy status

build:
	@echo "$(YELLOW)🚀 Building GoVerify Engine images...$(NC)"
	docker build -t goverify-engine-ai-service:latest ./ai-service
	docker build -f deploy/Dockerfile.api -t goverify-engine-kyc-api:latest .
	docker build -f deploy/Dockerfile.worker -t goverify-engine-kyc-worker:latest .
	@echo "$(GREEN)✅ All images built successfully!$(NC)"


deploy:
	@echo "$(YELLOW)📦 Deploying to Kubernetes...$(NC)"
	helm upgrade --install $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(NAMESPACE)
	@echo "$(YELLOW)⏳ Waiting for pods to be ready (this may take a few minutes)...$(NC)"
	kubectl wait --for=condition=ready pod -l app=etcd --timeout=300s || true
	kubectl wait --for=condition=ready pod -l app=kafka --timeout=300s || true
	kubectl wait --for=condition=ready pod -l app=milvus --timeout=300s || true
	@echo "$(GREEN)✅ Deployment finished!$(NC)"

status:
	@echo "$(BLUE)📊 System Status:$(NC)"
	@kubectl get pods -o wide
	@echo ""
	@echo "$(BLUE)🔗 Service Endpoints:$(NC)"
	@kubectl get svc | grep -E "LoadBalancer|ClusterIP"

logs:
	@kubectl logs -l app=kyc-api -f --tail=100

test:
	@echo "$(YELLOW)🧪 Running AI Service Tests...$(NC)"
	python3 ai-service/test_api.py

clean:
	@echo "$(RED)🗑️ Uninstalling GoVerify Engine...$(NC)"
	helm uninstall $(HELM_RELEASE_NAME)
