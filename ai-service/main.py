import base64
import numpy as np
import cv2
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from insightface.app import FaceAnalysis
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="GoVerify AI Service (InsightFace)")

# Initialize InsightFace
# buffalo_l is the model pack that includes 512-dim embedding model
face_app = FaceAnalysis(name='buffalo_l', providers=['CPUExecutionProvider'])
face_app.prepare(ctx_id=0, det_size=(640, 640))

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
    return {"status": "ok", "library": "insightface", "model": "buffalo_l"}

@app.post("/represent")
async def get_embedding(request: EmbeddingRequest):
    try:
        img = decode_base64_image(request.image_base64)
        
        faces = face_app.get(img)
        
        if not faces:
            raise HTTPException(status_code=400, detail="No face detected")
            
        # Return the embedding of the first face found (highest score/size usually)
        # InsightFace embeddings are already 512-dim float32
        embedding = faces[0].normed_embedding.tolist()
        
        return {"embedding": embedding}
        
    except Exception as e:
        if isinstance(e, HTTPException):
            raise e
        logger.error(f"Error generating embedding: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/verify")
async def verify_faces(request: VerifyRequest):
    try:
        img1 = decode_base64_image(request.img1_base64)
        img2 = decode_base64_image(request.img2_base64)
        
        faces1 = face_app.get(img1)
        faces2 = face_app.get(img2)
        
        if not faces1 or not faces2:
            raise HTTPException(status_code=400, detail="Face not detected in one or both images")
        
        emb1 = faces1[0].normed_embedding
        emb2 = faces2[0].normed_embedding
        
        # Calculate cosine similarity (dot product for normalized vectors)
        similarity = float(np.dot(emb1, emb2))
        
        # Distance is 1 - similarity for cosine
        distance = 1.0 - similarity
        
        # Typical threshold for buffalo_l is around 0.4 for cosine similarity (or higher)
        # But we'll return the metrics and let the caller decide
        is_same_person = bool(similarity > 0.4)
        
        return {
            "verified": is_same_person,
            "distance": distance,
            "similarity_score": similarity * 100,
            "library": "insightface",
            "model": "buffalo_l"
        }
        
    except Exception as e:
        if isinstance(e, HTTPException):
            raise e
        logger.error(f"Error during verification: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=5000)
