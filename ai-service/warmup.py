import numpy as np
import logging
from insightface.app import FaceAnalysis

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def warmup():
    print("Warming up InsightFace models (buffalo_l)...")
    try:
        # Initialize FaceAnalysis with the buffalo_l model pack
        # This will trigger the download of the models (det_10g, land5, w600k, etc.)
        app = FaceAnalysis(name='buffalo_l', providers=['CPUExecutionProvider'])
        app.prepare(ctx_id=0, det_size=(640, 640))
        
        # Create a dummy image to verify it works
        img = np.zeros((640, 640, 3), dtype=np.uint8)
        app.get(img)
        
        print("Warmup successful. All models downloaded and initialized.")
    except Exception as e:
        print(f"Warmup failed: {e}")
        # We don't want to fail the build if it's just a network issue, 
        # but for a production build it should probably fail.
        # raise e 

if __name__ == "__main__":
    warmup()
