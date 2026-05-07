FROM python:3.10-slim

WORKDIR /app

# Install system dependencies for OpenCV and DeepFace
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    libgomp1 \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Pre-download models to avoid download at runtime
# This is a bit tricky with DeepFace as it downloads on first use.
# We can run a small script to trigger the download.
COPY . .

# Warm up the models (download them during build)
# Using a dummy image to trigger download of retinaface and facenet512
RUN python warmup.py

EXPOSE 5000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "5000"]
