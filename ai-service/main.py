import base64
import numpy as np
import cv2
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from insightface.app import FaceAnalysis
from sentence_transformers import SentenceTransformer
from prometheus_fastapi_instrumentator import Instrumentator
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="GoVerify AI Service (InsightFace)")

# Initialize Prometheus Instrumentator
Instrumentator().instrument(app).expose(app)

# Initialize InsightFace
# buffalo_l is the model pack that includes 512-dim embedding model
face_app = FaceAnalysis(name="buffalo_l", providers=["CPUExecutionProvider"])
face_app.prepare(ctx_id=0, det_size=(640, 640))

MIN_FACE_PX = 80  # Minimum face width/height in pixels
MIN_DET_SCORE = 0.6  # Minimum detection confidence
VERIFY_THRESHOLD = 0.4  # Cosine similarity threshold for same-person
EMBEDDING_DIM = 512  # Expected embedding dimension for buffalo_l

# Initialize BERT for Name Embeddings
logger.info("Loading BERT model: l3cube-pune/indic-sentence-bert-nli")
name_model = SentenceTransformer("l3cube-pune/indic-sentence-bert-nli")
NAME_EMBEDDING_DIM = 768


class EmbeddingRequest(BaseModel):
    image_base64: str


class VerifyRequest(BaseModel):
    img1_base64: str
    img2_base64: str


class NameEmbeddingRequest(BaseModel):
    text: str


def decode_base64_image(base64_str: str):
    """Decode a base64-encoded image string to a cv2 BGR image."""
    try:
        if "," in base64_str:
            base64_str = base64_str.split(",")[1]
        img_data = base64.b64decode(base64_str)
        nparr = np.frombuffer(img_data, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        if img is None:
            raise ValueError(
                "Failed to decode image — unsupported format or corrupt data"
            )
        return img
    except Exception as e:
        logger.error(f"Image decode error: {e}")
        raise HTTPException(status_code=400, detail=f"Invalid image data: {str(e)}")


def extract_best_face(faces, context: str = ""):
    """
    From a list of detected faces:
    - Filter by minimum detection score
    - Filter by minimum bounding box size
    - Return the largest valid face (by area) — best practice for KYC
    Raises HTTPException with a descriptive message on failure.
    """
    if not faces:
        raise HTTPException(
            status_code=400,
            detail=f"No face detected{' in ' + context if context else ''}",
        )

    valid_faces = [f for f in faces if f.det_score > MIN_DET_SCORE]
    if not valid_faces:
        raise HTTPException(
            status_code=400,
            detail=f"No face met minimum confidence ({MIN_DET_SCORE}){' in ' + context if context else ''}",
        )

    # Filter by minimum pixel size — tiny faces produce unreliable embeddings
    sized_faces = [
        f
        for f in valid_faces
        if (f.bbox[2] - f.bbox[0]) >= MIN_FACE_PX
        and (f.bbox[3] - f.bbox[1]) >= MIN_FACE_PX
    ]
    if not sized_faces:
        raise HTTPException(
            status_code=400,
            detail=f"Face too small; minimum {MIN_FACE_PX}x{MIN_FACE_PX}px required{' in ' + context if context else ''}",
        )

    if len(sized_faces) > 1:
        logger.warning(
            f"{len(sized_faces)} valid faces detected{' in ' + context if context else ''}; selecting largest"
        )

    # Pick largest face by bounding box area — the subject in KYC is almost always the largest
    best = max(
        sized_faces, key=lambda f: (f.bbox[2] - f.bbox[0]) * (f.bbox[3] - f.bbox[1])
    )
    return best


def get_embedding_from_face(face) -> np.ndarray:
    """
    Extract and validate the normalized float32 embedding from a face object.
    normed_embedding is L2-normalized — correct for cosine similarity.
    """
    emb = face.normed_embedding.astype(np.float32)
    if emb.shape[0] != EMBEDDING_DIM:
        raise HTTPException(
            status_code=500,
            detail=f"Unexpected embedding dimension: {emb.shape[0]}, expected {EMBEDDING_DIM}",
        )
    return emb


@app.get("/health")
def health_check():
    return {
        "status": "ok",
        "library": "insightface",
        "model": "buffalo_l",
        "embedding_dim": EMBEDDING_DIM,
    }


@app.post("/represent")
async def get_embedding(request: EmbeddingRequest):
    try:
        img = decode_base64_image(request.image_base64)
        faces = face_app.get(img)
        best_face = extract_best_face(faces)
        emb = get_embedding_from_face(best_face)

        face_w = int(best_face.bbox[2] - best_face.bbox[0])
        face_h = int(best_face.bbox[3] - best_face.bbox[1])

        return {
            "embedding": emb.tolist(),  # float32 precision preserved
            "det_score": float(best_face.det_score),
            "face_size": [face_w, face_h],
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Unexpected error in /represent")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/represent-name")
async def get_name_embedding(request: NameEmbeddingRequest):
    try:
        if not request.text or not request.text.strip():
            raise HTTPException(status_code=400, detail="Text is required")

        # Generate embedding
        # encode() returns a numpy array
        emb = name_model.encode(request.text).astype(np.float32)

        if emb.shape[0] != NAME_EMBEDDING_DIM:
            raise HTTPException(
                status_code=500,
                detail=f"Unexpected name embedding dimension: {emb.shape[0]}, expected {NAME_EMBEDDING_DIM}",
            )

        return {
            "embedding": emb.tolist(),
            "dim": NAME_EMBEDDING_DIM,
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Unexpected error in /represent-name")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=5000)
