import requests
import base64
import json
import time
import sys

API_URL = "http://localhost:8080"
TOKEN = "my-test-token"

def enroll(image_path, name, dob, gender):
    with open(image_path, "rb") as f:
        img_base64 = base64.b64encode(f.read()).decode("utf-8")
    
    payload = {
        "photo_base64": img_base64,
        "name": name,
        "dob": dob,
        "gender": gender
    }
    
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=headers)
    return resp.json()

def verify(image_path, name, dob, gender):
    with open(image_path, "rb") as f:
        img_base64 = base64.b64encode(f.read()).decode("utf-8")
    
    payload = {
        "photo_base64": img_base64,
        "name": name,
        "dob": dob,
        "gender": gender
    }
    
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    resp = requests.post(f"{API_URL}/kyc/verify", json=payload, headers=headers)
    return resp.json()

def get_status(txn_id):
    headers = {"Authorization": f"Bearer {TOKEN}"}
    resp = requests.get(f"{API_URL}/kyc/status/{txn_id}", headers=headers)
    return resp.json()

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 test_with_images.py <image_path>")
        sys.exit(1)
    
    img_path = sys.argv[1]
    
    print(f"--- Enrolling user with image: {img_path} ---")
    enroll_resp = enroll(img_path, "Jane Doe", "1990-01-01", "FEMALE")
    print(f"Enroll Response: {json.dumps(enroll_resp, indent=2)}")
    
    txn_id = enroll_resp["transaction_id"]
    print(f"Waiting for enrollment processing (10s)...")
    time.sleep(10)
    
    status = get_status(txn_id)
    print(f"Enroll Status: {json.dumps(status, indent=2)}")
    
    if status.get("status") == "SUCCESS":
        print(f"\n--- Verifying user with same image ---")
        verify_resp = verify(img_path, "Jane Doe", "1990-01-01", "FEMALE")
        v_txn_id = verify_resp["transaction_id"]
        
        print(f"Waiting for verification processing (10s)...")
        time.sleep(10)
        
        v_status = get_status(v_txn_id)
        print(f"Verification Result: {json.dumps(v_status, indent=2)}")
    else:
        print("Enrollment failed or still pending. Check logs.")
