import os
import base64
import numpy as np
import cv2
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from deepface import DeepFace
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="GoVerify AI Service")

# Model configuration
MODEL_NAME = "Facenet512"
DETECTOR_BACKEND = "retinaface"
DISTANCE_METRIC = "cosine"

class EmbeddingRequest(BaseModel):
    image_base64: str

class VerifyRequest(BaseModel):
    img1_base64: str
    img2_base64: str

def decode_base64_image(base64_str: str):
    try:
        # Remove header if present
        if "," in base64_str:
            base64_str = base64_str.split(",")[1]
        
        img_data = base64.b64decode(base64_str)
        nparr = np.frombuffer(img_data, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        if img is None:
            raise ValueError("Failed to decode image")
        return img
    except Exception as e:
        logger.error(f"Error decoding image: {e}")
        raise HTTPException(status_code=400, detail=f"Invalid image data: {str(e)}")

@app.get("/health")
def health_check():
    return {"status": "ok", "model": MODEL_NAME}

@app.post("/represent")
async def get_embedding(request: EmbeddingRequest):
    try:
        img = decode_base64_image(request.image_base64)
        
        # DeepFace.represent returns a list of dictionaries (one for each face found)
        results = DeepFace.represent(
            img_path=img,
            model_name=MODEL_NAME,
            detector_backend=DETECTOR_BACKEND,
            enforce_detection=True,
            align=True
        )
        
        if not results:
            raise HTTPException(status_code=400, detail="No face detected")
            
        # Return the embedding of the first face found
        embedding = results[0]["embedding"]
        return {"embedding": embedding}
        
    except Exception as e:
        logger.error(f"Error generating embedding: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/verify")
async def verify_faces(request: VerifyRequest):
    try:
        img1 = decode_base64_image(request.img1_base64)
        img2 = decode_base64_image(request.img2_base64)
        
        result = DeepFace.verify(
            img1_path=img1,
            img2_path=img2,
            model_name=MODEL_NAME,
            detector_backend=DETECTOR_BACKEND,
            distance_metric=DISTANCE_METRIC,
        )
        
        # Extract metrics as in user's reference
        is_same_person = result["verified"]
        distance = result["distance"]
        similarity_percentage = (1 - distance) * 100
        
        return {
            "verified": is_same_person,
            "distance": distance,
            "similarity_score": similarity_percentage,
            "model": MODEL_NAME,
            "detector": DETECTOR_BACKEND
        }
        
    except Exception as e:
        logger.error(f"Error during verification: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=5000)
