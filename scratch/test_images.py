import requests
import base64
import json
import time
import sys

API_URL = "http://localhost:8080"

def login(username, password):
    payload = {"username": username, "password": password}
    resp = requests.post(f"{API_URL}/auth/login", json=payload)
    if resp.status_code == 200:
        return resp.json()["access_token"]
    else:
        print(f"Login failed: {resp.text}")
        sys.exit(1)

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
    if resp.status_code not in [200, 202]:
        print(f"Enrollment failed: {resp.text}")
        sys.exit(1)
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
    if resp.status_code not in [200, 202]:
        print(f"Verification request failed: {resp.text}")
        sys.exit(1)
    return resp.json()

def get_status(txn_id):
    headers = {"Authorization": f"Bearer {TOKEN}"}
    resp = requests.get(f"{API_URL}/kyc/status/{txn_id}", headers=headers)
    return resp.json()

if __name__ == "__main__":
    TOKEN = login("admin", "password123")
    
    name, dob, gender = "Person Three", "1990-01-01", "MALE"
    
    print(f"--- Enrolling {name} with p1a.png ---")
    enroll_resp = enroll("p1a.png", name, dob, gender)
    txn_id = enroll_resp["transaction_id"]
    
    print(f"Transaction ID: {txn_id}. Waiting for completion...")
    status = "PENDING"
    for _ in range(10):
        res = get_status(txn_id)
        status = res.get("status")
        if status != "PENDING":
            break
        time.sleep(2)
    
    if status != "SUCCESS":
        print(f"Enrollment failed with status: {status}")
        sys.exit(1)
    
    print("Enrollment SUCCESS. Proceeding to verification...")
    
    print(f"--- Verifying {name} with p1b.png ---")
    verify_resp = verify("p1b.png", name, dob, gender)
    v_txn_id = verify_resp["transaction_id"]
    
    print(f"Verification ID: {v_txn_id}. Waiting for result...")
    v_res = {}
    for _ in range(10):
        v_res = get_status(v_txn_id)
        if v_res.get("status") != "PENDING":
            break
        time.sleep(2)
    
    print("\n--- Test Results ---")
    print(json.dumps(v_res, indent=2))
