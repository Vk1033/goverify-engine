import base64
import requests
import numpy as np
import cv2

def test_health():
    resp = requests.get("http://localhost:5000/health")
    print(f"Health check: {resp.status_code}, {resp.json()}")

def test_represent():
    # Create a dummy image
    img = np.zeros((112, 112, 3), dtype=np.uint8)
    cv2.rectangle(img, (20, 20), (80, 80), (255, 255, 255), -1)
    _, buffer = cv2.imencode(".jpg", img)
    img_base64 = base64.b64encode(buffer).decode("utf-8")
    
    resp = requests.post("http://localhost:5000/represent", json={"image_base64": img_base64})
    print(f"Represent: {resp.status_code}")
    if resp.status_code == 200:
        emb = resp.json()["embedding"]
        print(f"Embedding length: {len(emb)}")
        print(f"First 5 values: {emb[:5]}")
    else:
        print(f"Error: {resp.json()}")

if __name__ == "__main__":
    # This script assumes the server is running
    try:
        test_health()
        test_represent()
    except Exception as e:
        print(f"Connection error: {e}")
