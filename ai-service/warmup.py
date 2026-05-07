from deepface import DeepFace
import numpy as np
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def warmup():
    print("Warming up DeepFace models (Facenet512, RetinaFace)...")
    # Create a dummy image
    img = np.zeros((112, 112, 3), dtype=np.uint8)
    try:
        # This will trigger the download of the models
        DeepFace.represent(
            img_path=img, 
            model_name='Facenet512', 
            detector_backend='retinaface', 
            enforce_detection=False
        )
        print("Warmup successful.")
    except Exception as e:
        print(f"Warmup skipped or failed: {e}")

if __name__ == "__main__":
    warmup()
