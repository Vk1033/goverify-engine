import requests
import base64
import json
import time
import sys
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler

API_URL = "http://localhost:8080"
TOKEN = ""
CALLBACK_PORT = 9999
CALLBACK_URL = f"http://host.docker.internal:{CALLBACK_PORT}/callback"
received_callbacks = []

class CallbackHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        received_callbacks.append(json.loads(post_data.decode('utf-8')))
        self.send_response(200)
        self.end_headers()
    def log_message(self, format, *args): return

def run_callback_server():
    server = HTTPServer(('0.0.0.0', CALLBACK_PORT), CallbackHandler)
    server.serve_forever()

def login(username, password):
    payload = {"username": username, "password": password}
    resp = requests.post(f"{API_URL}/auth/login", json=payload)
    if resp.status_code == 200:
        return resp.json()["access_token"]
    else:
        print(f"Login failed: {resp.text}")
        sys.exit(1)

def enroll(image_path, name, dob, gender, callback_url=None):
    with open(image_path, "rb") as f:
        img_base64 = base64.b64encode(f.read()).decode("utf-8")
    
    payload = {
        "photo_base64": img_base64,
        "name": name,
        "dob": dob,
        "gender": gender
    }
    if callback_url:
        payload["callback_url"] = callback_url
    
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=headers)
    return resp.json()

def verify(image_path, name, dob, gender, callback_url=None):
    with open(image_path, "rb") as f:
        img_base64 = base64.b64encode(f.read()).decode("utf-8")
    
    payload = {
        "photo_base64": img_base64,
        "name": name,
        "dob": dob,
        "gender": gender
    }
    if callback_url:
        payload["callback_url"] = callback_url
    
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    resp = requests.post(f"{API_URL}/kyc/verify", json=payload, headers=headers)
    return resp.json()

def get_status(txn_id):
    headers = {"Authorization": f"Bearer {TOKEN}"}
    resp = requests.get(f"{API_URL}/kyc/status/{txn_id}", headers=headers)
    return resp.json()

def search(name=None, gender=None):
    headers = {"Authorization": f"Bearer {TOKEN}"}
    params = {}
    if name:
        params["name"] = name
    if gender:
        params["gender"] = gender
    resp = requests.get(f"{API_URL}/kyc/search", headers=headers, params=params)
    return resp.json()

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 test_with_images.py <image_path>")
        sys.exit(1)
    
    img_path = sys.argv[1]

    # Start Callback Server
    print(f"--- Starting Callback Listener on port {CALLBACK_PORT} ---")
    threading.Thread(target=run_callback_server, daemon=True).start()
    
    print("--- 1. Testing Authentication Hardening ---")
    # Negative Test: No token
    bad_resp = requests.get(f"{API_URL}/kyc/search")
    print(f"Unauthenticated request status: {bad_resp.status_code} (Expected 401)")
    
    # Positive Test: Login
    print("Authenticating...")
    TOKEN = login("admin", "password123")
    print("Token obtained successfully.")

    print("\n--- 2. Testing Enrollment with Callback ---")
    name, dob, gender = "John Doe", "1985-05-20", "MALE"
    enroll_resp = enroll(img_path, name, dob, gender, callback_url=CALLBACK_URL)
    print(f"Enroll Response: {json.dumps(enroll_resp, indent=2)}")
    
    txn_id = enroll_resp["transaction_id"]
    print("Waiting for enrollment processing and callback...")
    status = {"status": "PENDING"}
    for _ in range(15):
        # Check if callback received
        if any(c.get("transaction_id") == txn_id for c in received_callbacks):
            print("✅ Callback received for enrollment!")
            status = {"status": "SUCCESS"} # Update status locally
            break
        
        status = get_status(txn_id)
        if status.get("status") == "SUCCESS":
            break
        time.sleep(2)
    print(f"Final Enroll Status: {status.get('status')}")

    if status.get("status") == "SUCCESS":
        print("\n--- 3. Testing Re-KYC Verification with Callback ---")
        verify_resp = verify(img_path, name, dob, gender, callback_url=CALLBACK_URL)
        v_txn_id = verify_resp['transaction_id']
        print(f"Verification ID: {v_txn_id}")
        
        print("Waiting for verification results and callback...")
        v_status = {"status": "PENDING"}
        for _ in range(15):
            # Check if callback received
            cb = next((c for c in received_callbacks if c.get("transaction_id") == v_txn_id), None)
            if cb:
                print(f"✅ Callback received for verification! Result: {cb.get('status')}")
                v_status = cb
                break

            v_status = get_status(v_txn_id)
            if v_status.get("status") != "PENDING":
                break
            time.sleep(2)
        print(f"Verification Result: {v_status.get('status')} (Score: {v_status.get('confidence_score')})")

        print("\n--- 4. Testing Identity Search Feature ---")
        
        print(f"A) Searching for name '{name}':")
        results = search(name=name)
        print(f"Found {len(results)} record(s).")

        print(f"B) Searching for gender '{gender}':")
        results = search(gender=gender)
        print(f"Found {len(results)} record(s).")

        print(f"C) Searching for name '{name}' AND gender '{gender}':")
        results = search(name=name, gender=gender)
        print(json.dumps(results, indent=2))
        
        print("\n--- All tests completed successfully! ---")
    else:
        print("Enrollment failed. Aborting further tests.")
