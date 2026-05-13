# Stage 1: Build & Download dependencies
FROM python:3.12-slim AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Force CPU-only versions of Torch and ONNX Runtime to save ~2GB
RUN pip install --no-cache-dir \
    torch --extra-index-url https://download.pytorch.org/whl/cpu \
    onnxruntime

# Install project dependencies
COPY pyproject.toml .
RUN pip install --no-cache-dir .

# Pre-download models (InsightFace & BERT)
COPY warmup.py .
RUN python warmup.py

# Stage 2: Final Lean Runtime Image
FROM python:3.12-slim

WORKDIR /app

# Install only the necessary runtime system libraries
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    libgomp1 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy installed Python packages from builder
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy pre-downloaded models from builder
# InsightFace models
COPY --from=builder /root/.insightface /root/.insightface
# HuggingFace/Sentence-Transformers models
COPY --from=builder /root/.cache /root/.cache

# Copy application code
COPY . .

EXPOSE 5000

# Use uvicorn to serve the FastAPI app
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "5000"]
