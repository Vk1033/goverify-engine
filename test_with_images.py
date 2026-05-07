import requests
import base64
import json
import time
import sys
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler

API_URL = "http://localhost:8080"
CALLBACK_PORT = 9999
CALLBACK_URL = f"http://host.docker.internal:{CALLBACK_PORT}/callback"

class CallbackHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        data = json.loads(post_data.decode('utf-8'))
        self.server.received_callbacks.append(data)
        self.send_response(200)
        self.end_headers()
    def log_message(self, format, *args): return

class CallbackServer(HTTPServer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.received_callbacks = []

class KYCTester:
    def __init__(self):
        self.token = ""
        self.callback_server = CallbackServer(('0.0.0.0', CALLBACK_PORT), CallbackHandler)
        self.server_thread = threading.Thread(target=self.callback_server.serve_forever, daemon=True)
        self.server_thread.start()
        print(f"--- Callback Listener started on port {CALLBACK_PORT} ---")

    def login(self, username, password):
        payload = {"username": username, "password": password}
        resp = requests.post(f"{API_URL}/auth/login", json=payload)
        if resp.status_code == 200:
            self.token = resp.json()["access_token"]
            return True
        return False

    def get_headers(self):
        return {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}

    def enroll(self, image_path, name, dob, gender, callback_url=None):
        try:
            with open(image_path, "rb") as f:
                img_base64 = base64.b64encode(f.read()).decode("utf-8")
        except FileNotFoundError:
            return {"error": f"File {image_path} not found"}
        
        payload = {"photo_base64": img_base64, "name": name, "dob": dob, "gender": gender}
        if callback_url: payload["callback_url"] = callback_url
        
        resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=self.get_headers())
        return resp.json(), resp.status_code

    def verify(self, image_path, name, dob, gender, callback_url=None):
        try:
            with open(image_path, "rb") as f:
                img_base64 = base64.b64encode(f.read()).decode("utf-8")
        except FileNotFoundError:
            return {"error": f"File {image_path} not found"}
        
        payload = {"photo_base64": img_base64, "name": name, "dob": dob, "gender": gender}
        if callback_url: payload["callback_url"] = callback_url
        
        resp = requests.post(f"{API_URL}/kyc/verify", json=payload, headers=self.get_headers())
        return resp.json(), resp.status_code

    def get_status(self, txn_id):
        resp = requests.get(f"{API_URL}/kyc/status/{txn_id}", headers=self.get_headers())
        return resp.json(), resp.status_code

    def search(self, name=None, gender=None):
        params = {}
        if name: params["name"] = name
        if gender: params["gender"] = gender
        resp = requests.get(f"{API_URL}/kyc/search", headers=self.get_headers(), params=params)
        return resp.json(), resp.status_code

    def wait_for_callback(self, txn_id, timeout=30):
        start_time = time.time()
        while time.time() - start_time < timeout:
            for cb in self.callback_server.received_callbacks:
                if cb.get("transaction_id") == txn_id:
                    return cb
            time.sleep(1)
        return None

    def wait_for_status(self, txn_id, timeout=30):
        start_time = time.time()
        while time.time() - start_time < timeout:
            status_resp, code = self.get_status(txn_id)
            if code == 200 and status_resp.get("status") not in ["PENDING", "QUEUED"]:
                return status_resp
            time.sleep(2)
        return None

def run_tests():
    tester = KYCTester()

    print("\n--- 1. Authentication & Health Tests ---")
    # Health Check
    health_resp = requests.get(f"{API_URL}/health")
    print(f"[Health] Status: {health_resp.status_code}, Body: {health_resp.json()}")
    
    # Unauthorized Access
    resp = requests.get(f"{API_URL}/kyc/search")
    print(f"[Auth] Unauthenticated search: {resp.status_code} (Expected 401)")

    # Login
    if tester.login("admin", "password123"):
        print("[Auth] Login successful.")
    else:
        print("[Auth] Login failed!")
        return

    print("\n--- 2. Enrollment Tests ---")
    # Case 2.1: Successful Enrollment (Person 1)
    print("Test 2.1: Enrolling Person 1 (p1a.png)...")
    res, code = tester.enroll("p1a.png", "John Doe", "1990-01-01", "MALE", callback_url=CALLBACK_URL)
    if code == 202:
        txn_id = res["transaction_id"]
        print(f"  Transaction ID: {txn_id}")
        cb = tester.wait_for_callback(txn_id)
        if cb:
            print(f"  ✅ Callback received: {cb['status']}")
        else:
            print("  ❌ Callback timed out. Polling status...")
            status = tester.wait_for_status(txn_id)
            print(f"  Status: {status.get('status') if status else 'Timeout'}")
    else:
        print(f"  ❌ Enrollment failed: {res}")

    # Case 2.2: Duplicate Enrollment (Same person, same name/dob)
    # Depending on implementation, this might be allowed or rejected. 
    # Let's see how the system handles it.
    print("Test 2.2: Enrolling Person 1 again (p1a.png)...")
    res, code = tester.enroll("p1a.png", "John Doe", "1990-01-01", "MALE")
    print(f"  Response Code: {code}, Body: {res}")

    # Case 2.3: Invalid Enrollment (Missing Name)
    print("Test 2.3: Enrolling with missing name...")
    payload = {"photo_base64": "SGVsbG8=", "dob": "1990-01-01", "gender": "MALE"}
    resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=tester.get_headers())
    print(f"  Response Code: {resp.status_code} (Expected 400)")

    print("\n--- 3. Verification Tests ---")
    # Case 3.1: Identity Match (p1a vs p1a)
    print("Test 3.1: Verifying Person 1 (p1a vs p1a)...")
    res, code = tester.verify("p1a.png", "John Doe", "1990-01-01", "MALE", callback_url=CALLBACK_URL)
    if code == 202:
        v_txn_id = res["transaction_id"]
        cb = tester.wait_for_callback(v_txn_id)
        if cb:
            print(f"  Result: {cb.get('status')} (Score: {cb.get('confidence_score')})")
        else:
            print("  ❌ Callback timeout.")
    else:
        print(f"  ❌ Verification failed: {res}")

    # Case 3.2: Cross-Image Match (p1a vs p1b)
    print("Test 3.2: Verifying Person 1 with different photo (p1a vs p1b)...")
    res, code = tester.verify("p1b.png", "John Doe", "1990-01-01", "MALE", callback_url=CALLBACK_URL)
    if code == 202:
        v_txn_id = res["transaction_id"]
        cb = tester.wait_for_callback(v_txn_id)
        if cb:
            print(f"  Result: {cb.get('status')} (Score: {cb.get('confidence_score')})")
        else:
            print("  ❌ Callback timeout.")
    else:
        print(f"  ❌ Verification failed: {res}")

    # Case 3.3: Different Person (p1a vs p2a)
    print("Test 3.3: Verifying Person 1 against Person 2 (p1a vs p2a)...")
    res, code = tester.verify("p2a.png", "John Doe", "1990-01-01", "MALE", callback_url=CALLBACK_URL)
    if code == 202:
        v_txn_id = res["transaction_id"]
        cb = tester.wait_for_callback(v_txn_id)
        if cb:
            print(f"  Result: {cb.get('status')} (Score: {cb.get('confidence_score')})")
            if cb.get('details', {}).get('explanation'):
                print(f"  Explanation: {cb['details']['explanation']}")
        else:
            print("  ❌ Callback timeout.")
    else:
        print(f"  ❌ Verification failed: {res}")

    print("\n--- 4. Demographic Mismatch Tests ---")
    # Case 4.1: Face Match but Name Mismatch
    print("Test 4.1: Verifying Person 1 but with wrong name (Jane Doe)...")
    res, code = tester.verify("p1a.png", "Jane Doe", "1990-01-01", "MALE", callback_url=CALLBACK_URL)
    if code == 202:
        v_txn_id = res["transaction_id"]
        cb = tester.wait_for_callback(v_txn_id)
        print(f"  Result: {cb.get('status')} (Score: {cb.get('confidence_score')})")
        print(f"  Details: Name Sim: {cb.get('details', {}).get('name_similarity')}, Face Sim: {cb.get('details', {}).get('face_similarity')}")
    else:
        print(f"  ❌ Verification failed: {res}")

    print("\n--- 5. Search & Identity Management ---")
    # Case 5.1: Search by Name
    print("Test 5.1: Searching for 'John Doe'...")
    res, code = tester.search(name="John Doe")
    print(f"  Found {len(res)} results.")
    
    # Case 5.2: Search by Gender
    print("Test 5.2: Searching for MALE...")
    res, code = tester.search(gender="MALE")
    print(f"  Found {len(res)} results.")

    print("\n--- 6. Error & Boundary Tests ---")
    # Case 6.1: Status for non-existent transaction
    print("Test 6.1: Checking status for non-existent transaction...")
    res, code = tester.get_status("00000000-0000-0000-0000-000000000000")
    print(f"  Status Code: {code} (Expected 404)")

    # Case 6.2: Verify against non-existent enrollment
    print("Test 6.2: Verifying a person who was never enrolled...")
    res, code = tester.verify("p2a.png", "Ghost Person", "1900-01-01", "FEMALE", callback_url=CALLBACK_URL)
    if code == 202:
        v_txn_id = res["transaction_id"]
        cb = tester.wait_for_callback(v_txn_id)
        print(f"  Result: {cb.get('status')} (Expected NO_MATCH or FAILURE)")
    else:
        print(f"  Response: {code}, Body: {res}")

    print("\n--- All tests completed ---")

if __name__ == "__main__":
    run_tests()
