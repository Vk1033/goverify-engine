import numpy as np
import logging
from insightface.app import FaceAnalysis
from sentence_transformers import SentenceTransformer

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def warmup():
    print("Warming up InsightFace models (buffalo_l)...")
    try:
        # Initialize FaceAnalysis
        app = FaceAnalysis(name="buffalo_l", providers=["CPUExecutionProvider"])
        app.prepare(ctx_id=0, det_size=(640, 640))

        # Create a dummy image
        img = np.zeros((640, 640, 3), dtype=np.uint8)
        app.get(img)

        print("InsightFace warmup successful.")
    except Exception as e:
        print(f"InsightFace warmup failed: {e}")

    print("Warming up BERT model (l3cube-pune/indic-sentence-bert-nli)...")
    try:
        # Initialize BERT model - this triggers download
        SentenceTransformer("l3cube-pune/indic-sentence-bert-nli")
        print("BERT warmup successful.")
    except Exception as e:
        print(f"BERT warmup failed: {e}")


if __name__ == "__main__":
    warmup()
