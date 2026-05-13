# GoVerify Engine 🛡️

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![Milvus](https://img.shields.io/badge/VectorDB-Milvus-0288D1?style=for-the-badge)](https://milvus.io/)
[![Kafka](https://img.shields.io/badge/Messaging-Kafka-231F20?style=for-the-badge&logo=apachekafka)](https://kafka.apache.org/)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)

**GoVerify Engine** is a state-of-the-art, AI-powered identity verification (KYC) system designed for extreme scalability and precision. It leverages high-fidelity facial biometrics, semantic name matching, and a distributed event-driven architecture to provide instant, reliable Re-KYC capabilities.

---

## 🚀 Key Features

- **Biometric Identity**: InsightFace-powered facial embeddings (`buffalo_l`) for high-precision matching.
- **Semantic Verification**: Hybrid matching combining biometric similarity with BERT-based semantic analysis and Levenshtein syntactic checks.
- **Production-Ready Deployment**: Automated Kubernetes orchestration via **Helm** with resource limits and health checks.
- **Optimized AI Service**: Multi-stage Docker builds with pre-downloaded models for lightning-fast container startup.
- **Event-Driven Scaling**: Kafka-backed asynchronous processing to handle massive request spikes.
- **Ultra-Fast Search**: Milvus Vector DB for sub-millisecond retrieval across millions of identities.
- **Comprehensive Observability**: Native integration with Prometheus, Grafana, Jaeger, and Loki for full-stack visibility.
- **Enterprise Ready**: Built with `uber-go/fx` for clean dependency injection and modularity.

---

## 🏗️ System Architecture

The following diagram illustrates the high-concurrency architecture of the GoVerify Engine:

```mermaid
graph TD
    %% External
    Client([Client Application]) -->|REST API| API[KYC API]
    API -->|Auth/State| Redis[(Redis)]
    
    %% Orchestration
    API -->|Push Task| Kafka{Kafka}
    Kafka -->|Consume Task| Worker[KYC Worker]
    
    %% Processing
    Worker -->|Inference Requests| AI[AI Service]
    subgraph "AI Microservice (Python/FastAPI)"
        AI -->|InsightFace| FaceModel[Buffalo_L Embedding Model]
        AI -->|S-BERT| NameModel[Indic-Sentence-BERT]
    end
    
    Worker -->|Syntactic Match| Lev[Levenshtein Similarity]
    Worker -->|Semantic Match| BERT[BERT Name Embedding]
    
    %% Data Store
    Worker -->|Vector Search| Milvus[(Milvus Vector DB)]
    subgraph "Milvus Infrastructure"
        Milvus --> MinIO[(MinIO)]
        Milvus --> Etcd[(Etcd)]
    end
    
    %% Status & Callbacks
    Worker -->|Update Status| Redis
    Worker -->|Callback| Client
    
    %% Observability
    API -.->|Metrics| Prom[Prometheus]
    Worker -.->|Metrics| Prom
    
    API -.->|Traces| Jaeger[Jaeger]
    Worker -.->|Traces| Jaeger
    
    API -.->|Logs| Loki[Loki]
    Worker -.->|Logs| Loki
    
    Prom --> Grafana[Grafana Unified Dashboard]
    Jaeger --> Grafana
    Loki --> Grafana
```

---

## 🛠️ Tech Stack

| Category | Technology |
| :--- | :--- |
| **Core** | Golang 1.25, Python 3.12 |
| **Frameworks** | Gin (Go), FastAPI (Python), Uber-fx |
| **Messaging** | Apache Kafka |
| **Databases** | Milvus (Vector), Redis (Cache/Status) |
| **AI/ML** | InsightFace (Biometrics), BERT (Names) |
| **Observability** | Prometheus, Grafana, Jaeger, Loki |
| **Infrastructure** | Docker, Kubernetes, Helm |

---

## 🚦 Getting Started

### Prerequisites

- **Docker & Docker Compose** (Local development)
- **Kubernetes Cluster** & **Helm 3** (Production deployment)
- **Go 1.25+** & **Python 3.12+**

### Quick Start (Local Development)

1. **Clone the repository**:
   ```bash
   git clone https://github.com/vk1033/goverify-engine.git
   cd goverify-engine
   ```

2. **Spin up the infrastructure**:
   ```bash
   docker-compose up -d
   ```
   *Starts API, Worker, AI Service, Kafka, Milvus, Redis, and Observability stack.*

### Production Deployment (Kubernetes)

We provide a professional `Makefile` to manage the entire lifecycle:

```bash
# 1. Build optimized images (AI models pre-cached)
make build

# 2. Deploy to Kubernetes via Helm
make deploy

# 3. Check system status
make status
```

---

## 🛠️ Management & Automation

The included `Makefile` provides a standardized interface for common tasks:

| Command | Description |
| :--- | :--- |
| `make build` | Builds all Docker images (optimized for size and speed). |
| `make deploy` | Installs/Upgrades the Helm chart and waits for system readiness. |
| `make status` | Displays Kubernetes pod status and service endpoints. |
| `make logs` | Tails the logs for the KYC API service. |
| `make test` | Runs end-to-end validation of the AI service scoring. |
| `make clean` | Uninstalls the Helm release. |

---

## 📊 Observability & Monitoring

GoVerify provides deep visibility into your identity cluster. Access the dashboards at `http://localhost:3001` (default):

- **Home Dashboard**: High-level overview of system health and throughput.
- **K8s Cluster Dashboard**: Detailed resource usage for Kubernetes pods.
- **Jaeger Tracing**: Trace verification requests across microservices to identify bottlenecks.
- **Loki Logging**: Centralized log exploration for debugging and auditing.

---

## 🛡️ Security & Resilience

- **AI Model Safety**: AI services use CPU-optimized versions of Torch and ONNX Runtime to ensure stability in resource-constrained environments.
- **Data Privacy**: Sensitive demographic data is hashed using Argon2; PII is encrypted at rest using AES-256 GCM.
- **Resilience**: `initContainers` in Helm ensure that critical infrastructure (Kafka, Milvus) is fully ready before application services start.
- **Asynchronous Processing**: Kafka buffering ensures no identity data is lost during traffic spikes or worker maintenance.

---

## 🤝 Contributing

We welcome contributions! Please see our [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

