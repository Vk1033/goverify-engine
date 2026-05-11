import requests
import base64
import json
import time
import sys
import threading
import os
from http.server import HTTPServer, BaseHTTPRequestHandler

# Configuration
API_URL = os.getenv("API_URL", "http://localhost:8080")
CALLBACK_PORT = int(os.getenv("CALLBACK_PORT", 9999))
# host.docker.internal works when running worker in Docker and listener on Host
# For K8s on Linux, use the host IP reachable from the pods
CALLBACK_HOST = os.getenv("CALLBACK_HOST", "host.docker.internal")
CALLBACK_URL = f"http://{CALLBACK_HOST}:{CALLBACK_PORT}/callback"

# Base directory for images, relative to this script
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
IMAGE_DIR = os.path.join(SCRIPT_DIR, "../../testdata/images")


def get_image_path(filename):
    return os.path.join(IMAGE_DIR, filename)


class CallbackHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers["Content-Length"])
        post_data = self.rfile.read(content_length)
        try:
            data = json.loads(post_data.decode("utf-8"))
            self.server.received_callbacks.append(data)
            # print(f"\n[DEBUG] Callback received for txn: {data.get('transaction_id')}")
        except Exception as e:
            print(f"Error parsing callback: {e}")

        self.send_response(200)
        self.end_headers()

    def log_message(self, format, *args):
        pass  # Quiet logs


class CallbackServer(HTTPServer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.received_callbacks = []


class KYCTester:
    def __init__(self):
        self.token = ""
        try:
            self.callback_server = CallbackServer(
                ("0.0.0.0", CALLBACK_PORT), CallbackHandler
            )
            self.server_thread = threading.Thread(
                target=self.callback_server.serve_forever, daemon=True
            )
            self.server_thread.start()
            print(f"[*] Callback listener started on port {CALLBACK_PORT}")
            print(f"[*] Callback URL configured as: {CALLBACK_URL}")
        except Exception as e:
            print(f"[!] Failed to start callback server: {e}")
            sys.exit(1)

    def login(self, username="admin", password="password123"):
        print(f"[*] Logging in as {username}...")
        resp = requests.post(
            f"{API_URL}/auth/login", json={"username": username, "password": password}
        )
        if resp.status_code == 200:
            self.token = resp.json()["access_token"]
            return True
        return False

    def get_headers(self):
        return {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
        }

    def enroll(self, image_path, name, dob, gender):
        print(f"[*] Enrolling {name} ({image_path})...")
        with open(image_path, "rb") as f:
            img_base64 = base64.b64encode(f.read()).decode("utf-8")

        payload = {
            "photo_base64": img_base64,
            "name": name,
            "dob": dob,
            "gender": gender,
            "callback_url": CALLBACK_URL,
        }
        resp = requests.post(
            f"{API_URL}/kyc/enroll", json=payload, headers=self.get_headers()
        )
        resp.raise_for_status()
        return resp.json()["transaction_id"]

    def verify(self, image_path, name, dob, gender):
        print(f"[*] Verifying {name} ({image_path})...")
        with open(image_path, "rb") as f:
            img_base64 = base64.b64encode(f.read()).decode("utf-8")

        payload = {
            "photo_base64": img_base64,
            "name": name,
            "dob": dob,
            "gender": gender,
            "callback_url": CALLBACK_URL,
        }
        resp = requests.post(
            f"{API_URL}/kyc/verify", json=payload, headers=self.get_headers()
        )
        resp.raise_for_status()
        return resp.json()["transaction_id"]

    def wait_for_callback(self, txn_id, timeout=30):
        start_time = time.time()
        while time.time() - start_time < timeout:
            for i, cb in enumerate(self.callback_server.received_callbacks):
                if cb.get("transaction_id") == txn_id:
                    return self.callback_server.received_callbacks.pop(i)
            time.sleep(0.5)
        return None

    def search(self, name=None, gender=None):
        params = {}
        if name:
            params["name"] = name
        if gender:
            params["gender"] = gender
        resp = requests.get(
            f"{API_URL}/kyc/search", headers=self.get_headers(), params=params
        )
        return resp.json()


def run_suite():
    tester = KYCTester()

    if not tester.login():
        print("[!] Login failed. Is the API running?")
        return

    print("\n" + "=" * 50)
    print("PHASE 1: ENROLLMENT")
    print("=" * 50)

    # Enroll Person 1
    p1_txn = tester.enroll(get_image_path("p1a.png"), "John Doe", "1990-01-01", "MALE")
    cb1 = tester.wait_for_callback(p1_txn)
    if cb1:
        print(f"  [+] Person 1 enrolled. Status: {cb1['status']}")
    else:
        print("  [!] Timeout waiting for Person 1 enrollment callback")

    # Enroll Person 3
    p3_txn = tester.enroll(
        get_image_path("p3a.png"), "Alice Smith", "1985-05-20", "FEMALE"
    )
    cb3 = tester.wait_for_callback(p3_txn)
    if cb3:
        print(f"  [+] Person 3 enrolled. Status: {cb3['status']}")

    print("\n" + "=" * 50)
    print("PHASE 2: VERIFICATION (MATCH)")
    print("=" * 50)

    # Match Person 1 (p1a vs p1b)
    v1_txn = tester.verify(get_image_path("p1b.png"), "John Doe", "1990-01-01", "MALE")
    v_cb1 = tester.wait_for_callback(v1_txn)
    if v_cb1:
        print(f"  [+] P1 Result: {v_cb1['status']} (Score: {v_cb1['confidence_score']:.4f})")
        print(f"      Face: {v_cb1['details']['face_similarity']:.4f}, Name: {v_cb1['details']['name_similarity']:.4f}, Demo: {v_cb1['details']['demographic_match']}")
        print(f"      Reason: {v_cb1['details'].get('explanation', 'N/A')}")

    # Match Person 3 (p3a vs p3b)
    v3_txn = tester.verify(get_image_path("p3b.png"), "Alice Smith", "1985-05-20", "FEMALE")
    v_cb3 = tester.wait_for_callback(v3_txn)
    if v_cb3:
        print(f"  [+] P3 Result: {v_cb3['status']} (Score: {v_cb3['confidence_score']:.4f})")
        print(f"      Face: {v_cb3['details']['face_similarity']:.4f}, Name: {v_cb3['details']['name_similarity']:.4f}, Demo: {v_cb3['details']['demographic_match']}")
        print(f"      Reason: {v_cb3['details'].get('explanation', 'N/A')}")

    print("\n" + "=" * 50)
    print("PHASE 3: VERIFICATION (MISMATCH)")
    print("=" * 50)
    # Mismatch (p1a vs p2a)
    v_mismatch_txn = tester.verify(get_image_path("p2a.png"), "John Doe", "1990-01-01", "MALE")
    v_cb_m = tester.wait_for_callback(v_mismatch_txn)
    if v_cb_m:
        print(f"  [+] Mismatch Result: {v_cb_m['status']} (Score: {v_cb_m['confidence_score']:.4f})")
        print(f"      Face: {v_cb_m['details']['face_similarity']:.4f}, Name: {v_cb_m['details']['name_similarity']:.4f}, Demo: {v_cb_m['details']['demographic_match']}")
        print(f"      Reason: {v_cb_m['details'].get('explanation', 'N/A')}")

    print("\n" + "=" * 50)
    print("PHASE 4: DEMOGRAPHIC MISMATCH")
    print("=" * 50)
    v_demo_txn = tester.verify(get_image_path("p1b.png"), "Wrong Name", "1990-01-01", "MALE")
    v_cb_d = tester.wait_for_callback(v_demo_txn)
    if v_cb_d:
        print(f"  [+] Demo Result: {v_cb_d['status']} (Score: {v_cb_d['confidence_score']:.4f})")
        print(f"      Face: {v_cb_d['details']['face_similarity']:.4f}, Name: {v_cb_d['details']['name_similarity']:.4f}, Demo: {v_cb_d['details']['demographic_match']}")
        print(f"      Reason: {v_cb_d['details'].get('explanation', 'N/A')}")

    print("\n" + "=" * 50)
    print("PHASE 5: SEARCH")
    print("=" * 50)
    results = tester.search(name="John Doe")
    if results:
        print(f"  [+] Found {len(results)} records for 'John Doe'")

    print("\n" + "=" * 50)
    print("PHASE 6: NAME ORDER SWAP")
    print("=" * 50)
    v_swap_txn = tester.verify(get_image_path("p1b.png"), "Doe John", "1990-01-01", "MALE")
    v_cb_s = tester.wait_for_callback(v_swap_txn)
    if v_cb_s:
        print(f"  [+] Swap Result: {v_cb_s['status']} (Score: {v_cb_s['confidence_score']:.4f})")
        print(f"      Face: {v_cb_s['details']['face_similarity']:.4f}, Name: {v_cb_s['details']['name_similarity']:.4f}, Demo: {v_cb_s['details']['demographic_match']}")
        print(f"      Reason: {v_cb_s['details'].get('explanation', 'N/A')}")

    print("\n" + "=" * 50)
    print("PHASE 7: IDENTITY SANITY CHECK")
    print("=" * 50)
    v_veto_txn = tester.verify(get_image_path("p3a.png"), "John Doe", "1990-01-01", "MALE")
    v_cb_v = tester.wait_for_callback(v_veto_txn)
    if v_cb_v:
        print(f"  [+] Veto Result: {v_cb_v['status']} (Score: {v_cb_v['confidence_score']:.4f})")
        print(f"      Face: {v_cb_v['details']['face_similarity']:.4f}, Name: {v_cb_v['details']['name_similarity']:.4f}, Demo: {v_cb_v['details']['demographic_match']}")
        print(f"      Reason: {v_cb_v['details'].get('explanation', 'N/A')}")

    print("\n" + "=" * 50)
    print("TEST SUITE COMPLETED")
    print("=" * 50)


if __name__ == "__main__":
    run_suite()
