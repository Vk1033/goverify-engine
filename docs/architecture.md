# Architecture Overview

The GoVerify Engine is a high-performance, asynchronous identity verification system designed to handle biometric and semantic matching at scale.

## System Components

### 1. External Interface
- **KYC API (Go/Gin)**: The primary entry point for clients. Handles authentication (JWT), request validation, and asynchronous task orchestration.
- **Swagger UI**: Interactive API documentation.

### 2. Processing Core
- **KYC Worker (Go)**: The background engine that processes verification tasks. It coordinates between the AI service and the vector database. It performs **Multi-modal Verification** using biometric similarity (Milvus), semantic similarity (BERT), and multi-factor demographic matching (Argon2).
- **AI Service (Python/FastAPI)**: A specialized service for biometric and semantic heavy lifting.
  - **InsightFace**: Generates high-fidelity face embeddings (Buffalo_L).
  - **S-BERT**: Generates semantic embeddings for names to handle variations and transliterations.

### 3. Data & Orchestration
- **Kafka**: The message backbone. Ensures reliability and decoupling between API and Worker.
- **Milvus**: A state-of-the-art vector database for high-dimensional similarity search.
- **Redis**: Fast, in-memory storage for transaction status tracking and temporary state.

### 4. Observability
- **Prometheus & Grafana**: Real-time metrics and dashboards.
- **Jaeger**: Distributed tracing for identifying bottlenecks across microservices.
- **Loki**: Centralized log aggregation.

## Architecture Diagram

```mermaid
graph TD
    %% External
    Client([Client Application]) -->|REST API| API[KYC API]
    API -->|Auth/State| Redis[(Redis)]
    
    %% Orchestration
    API -->|1. Enqueue| Kafka{Kafka}
    Kafka -->|2. Consume| Worker[KYC Worker]
    
    %% Processing
<<<<<<< HEAD
    Worker -->|3. Inference| AI[AI Microservice]
    subgraph "AI Microservice (Python/FastAPI)"
        AI -->|InsightFace| FaceModel[Face Embedding]
        AI -->|S-BERT| NameModel[Name Embedding]
    end
    
    %% Business Logic
    subgraph "Go Processing Logic"
        Worker -->|Semantic Similarity| Cosine[Cosine Score]
        Worker -->|Identity Security| AES[AES-GCM / Argon2]
    end
=======
    Worker -->|Inference Requests| AI[AI Microservice]
    subgraph "AI Microservice (Python/FastAPI)"
        AI -->|InsightFace| FaceModel[Face Embedding Model]
        AI -->|S-BERT| NameModel[Name Embedding Model]
    end
    
    %% Business Logic
    Worker -->|Semantic Similarity| Cosine[Cosine Similarity]
    Worker -->|Identity Security| AES[AES-GCM / Argon2]
>>>>>>> a7a9b23 (docs: update architectural documentation and diagrams to reflect multi-modal verification and demographic hashing implementation.)
    
    %% Data Store
    Worker -->|4. Vector Search| Milvus[(Milvus Vector DB)]
    
    %% Status & Callbacks
    Worker -->|5. Update Result| Redis
    Worker -->|6. Callback| Client
    
    %% Observability
    API -.->|Metrics/Traces| Prom[Prometheus / Jaeger]
    Worker -.->|Metrics/Traces| Prom
    Prom --> Grafana[Grafana]
```

## Flow Description

1. **Request**: A client sends a `KYCRequest` (Photo, Name, DOB, etc.) to the `KYC API`.
2. **Acceptance**: The API validates the request, generates a `TransactionID`, stores the initial `PENDING` status in **Redis**, and pushes a message to **Kafka**.
3. **Response**: The API immediately returns the `TransactionID` to the client.
4. **Processing**: The **KYC Worker** picks up the message from Kafka.
5. **Embedding**: The Worker calls the **AI Service** to generate a 512-dimensional vector for the face and a 768-dimensional vector for the name.
6. **Matching**: The Worker:
   - Queries **Milvus** to find candidates based on face similarity.
   - Calculates **Semantic Similarity** (BERT) for names using cosine similarity.
   - Computes a final hybrid score (Biometric + Semantic + Demographic Hash).
7. **Completion**: The Worker updates the transaction status in **Redis** and triggers a webhook callback to the **Client** with the final `VerificationResult`.
