import base64
import requests
import numpy as np
# import cv2


def test_health():
    try:
        resp = requests.get("http://localhost:5000/health")
        print(f"Health check: {resp.status_code}, {resp.json()}")
    except Exception as e:
        print(f"Health check failed: {e}")


# def test_represent():
#     # Create a dummy image
#     img = np.zeros((640, 640, 3), dtype=np.uint8)
#     # Draw a face-like shape to help detection (though dummy might not always work)
#     cv2.rectangle(img, (200, 200), (440, 440), (255, 255, 255), -1)
#     _, buffer = cv2.imencode(".jpg", img)
#     img_base64 = base64.b64encode(buffer).decode("utf-8")

#     print("Testing /represent endpoint...")
#     try:
#         resp = requests.post("http://localhost:5000/represent", json={"image_base64": img_base64})
#         print(f"Represent status: {resp.status_code}")
#         if resp.status_code == 200:
#             emb = resp.json()["embedding"]
#             print(f"Embedding length: {len(emb)}")
#             print(f"First 5 values: {emb[:5]}")
#         else:
#             print(f"Error: {resp.text}")
#     except Exception as e:
#         print(f"Represent request failed: {e}")


def test_represent_name():
    print("Testing /represent-name endpoint...")
    try:
        resp = requests.post(
            "http://localhost:5000/represent-name", json={"text": "John Doe"}
        )
        print(f"Represent Name status: {resp.status_code}")
        if resp.status_code == 200:
            data = resp.json()
            emb = data["embedding"]
            print(f"Embedding length: {len(emb)}")
            print(f"Dim: {data.get('dim')}")
            print(f"First 5 values: {emb[:5]}")
        else:
            print(f"Error: {resp.text}")
    except Exception as e:
        print(f"Represent Name request failed: {e}")


if __name__ == "__main__":
    # This script assumes the server is running
    try:
        test_health()
        # test_represent()
        test_represent_name()
    except Exception as e:
        print(f"Connection error: {e}")
