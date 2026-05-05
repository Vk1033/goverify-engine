# KYC Visual Identity Engine

This is a scalable, AI-driven KYC system that creates a visual identity of a user using facial embeddings and demographic data, and enables instant re-verification (Re-KYC). Built for high-concurrency environments utilizing Golang, Kafka, Milvus, Redis, and Kubernetes.

## System Architecture

- **KYC API Gateway**: Receives REST requests (`/kyc/enroll`, `/kyc/verify`, `/kyc/status/:transaction_id`), assigns a unique Transaction ID, and pushes messages asynchronously to Kafka.
- **KYC Worker**: A robust consumer using `fx` lifecycle management that processes requests from Kafka, generates multi-modal embeddings (Face 512D, Name 768D), computes Argon2 demographic hashes, and inserts/verifies against the Milvus Vector Database.
- **Kafka**: Acts as the message broker for decoupling request ingestion and processing.
- **Milvus Vector DB**: Stores identities and performs high-speed cosine similarity matching for Face and Name embeddings.
- **Redis**: Functions as a fast, high-availability key-value store for polling transaction statuses (`GET /kyc/status/:transaction_id`).

## Tech Stack
- **Language**: Golang 1.22+
- **DI Framework**: `uber-go/fx`
- **REST framework**: `gin-gonic/gin`
- **Configuration**: `spf13/viper` & `spf13/cobra`
- **Vector DB**: `milvus-sdk-go/v2`
- **Messaging**: `segmentio/kafka-go`
- **Data store**: `go-redis/redis/v8`

## Quick Start (Docker Compose)

1. **Start Infrastructure**:
   ```bash
   docker-compose up -d zookeeper kafka etcd minio milvus-standalone redis
   ```
2. **Start Services**:
   ```bash
   docker-compose up -d kyc-api kyc-worker
   ```
3. **Verify API is running**:
   ```bash
   curl -H "Authorization: Bearer my-token" http://localhost:8080/kyc/status/dummy_txn
   ```

## Development & Testing

Ensure you have Go installed, then install dependencies:
```bash
go mod tidy
```

Start the API locally:
```bash
go run cmd/kyc-api/main.go
```

Start the Worker locally:
```bash
go run cmd/kyc-worker/main.go
```

## API Documentation

### POST `/kyc/enroll`
Enrolls a new user identity asynchronously.
**Request Body**:
```json
{
  "photo_base64": "<base64_string>",
  "name": "Jane Doe",
  "dob": "1990-01-01",
  "gender": "FEMALE"
}
```

### POST `/kyc/verify`
Verifies a returning user.
**Request Body**:
```json
{
  "photo_base64": "<base64_string>",
  "name": "Jane Doe",
  "dob": "1990-01-01",
  "gender": "FEMALE"
}
```

### GET `/kyc/status/:transaction_id`
Retrieves the async process result.

### GET `/kyc/search`
Searches for identities based on name/gender metadata.

## Interactive API Documentation
Swagger UI is available at: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

## Kubernetes Deployment (Helm)

To deploy the engine to a Kubernetes cluster using Helm:

1. **Build and push images**:
   ```bash
   docker build -t goverify-engine-api:latest -f deploy/Dockerfile.api .
   docker build -t goverify-engine-worker:latest -f deploy/Dockerfile.worker .
   # Push to your registry...
   ```

2. **Install using Helm**:
   ```bash
   helm install goverify ./deploy/helm/goverify
   ```

3. **Verify Deployment**:
   ```bash
   kubectl get pods -l app=kyc-api
   kubectl get pods -l app=kyc-worker
   ```
