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
        except Exception as e:
            print(f"[!] Failed to start callback server: {e}")
            sys.exit(1)

    def register(self, username="admin", password="password123"):
        resp = requests.post(
            f"{API_URL}/auth/register", json={"username": username, "password": password}
        )
        if resp.status_code in [201, 400]: # 201 Created or 400 (likely already exists)
            return True
        return False

    def login(self, username="admin", password="password123"):
        # Try to register first (ignore if already exists)
        self.register(username, password)
        
        resp = requests.post(
            f"{API_URL}/auth/login", json={"username": username, "password": password}
        )
        if resp.status_code == 200:
            data = resp.json()
            self.token = data["access_token"]
            return True
        return False

    def get_headers(self):
        return {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
        }

    def enroll(self, image_path, name, dob, gender):
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

    def wait_for_callback(self, txn_id, timeout=60):
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


def print_phase(name):
    print("\n" + "=" * 60)
    print(f" PHASE: {name}")
    print("=" * 60)


def print_result(txn_id, cb):
    if not cb:
        print(f"  [!] TIMEOUT for transaction {txn_id}")
        return
    status = cb.get("status", "UNKNOWN")
    score = cb.get("confidence_score", 0.0)
    details = cb.get("details", {})

    color = (
        "\033[92m"
        if status == "MATCHED"
        else "\033[93m"
        if status == "PARTIAL_MATCH"
        else "\033[91m"
    )
    reset = "\033[0m"

    print(f"  [+] Status: {color}{status}{reset} (Score: {score:.4f})")
    print(f"      Face Sim: {details.get('face_similarity', 0):.4f}")
    print(f"      Name Sim: {details.get('name_similarity', 0):.4f}")
    print(f"      Demo Match: {details.get('demographic_match', False)}")
    if details.get("explanation"):
        print(f"      Reason: {details['explanation']}")


def run_suite():
    tester = KYCTester()

    if not tester.login():
        print("[!] Login failed. Is the API running?")
        return

    # ---------------------------------------------------------
    print_phase("1. STANDARD ENROLLMENT")
    # ---------------------------------------------------------
    p1_txn = tester.enroll(get_image_path("p1a.png"), "John Doe", "1990-01-01", "MALE")
    print("[*] Enrolling John Doe (p1a)...")
    cb1 = tester.wait_for_callback(p1_txn)
    print_result(p1_txn, cb1)

    p3_txn = tester.enroll(
        get_image_path("p3a.png"), "Alice Smith", "1985-05-20", "FEMALE"
    )
    print("[*] Enrolling Alice Smith (p3a)...")
    cb3 = tester.wait_for_callback(p3_txn)
    print_result(p3_txn, cb3)

    # ---------------------------------------------------------
    print_phase("2. BIOMETRIC VERIFICATION (MATCH)")
    # ---------------------------------------------------------
    v1_txn = tester.verify(get_image_path("p1b.png"), "John Doe", "1990-01-01", "MALE")
    print("[*] Verifying John Doe (p1b)...")
    print_result(v1_txn, tester.wait_for_callback(v1_txn))

    # ---------------------------------------------------------
    print_phase("3. CROSS-PERSON VERIFICATION (FACE MISMATCH)")
    # ---------------------------------------------------------
    v_mismatch_txn = tester.verify(
        get_image_path("p2a.png"), "John Doe", "1990-01-01", "MALE"
    )
    print("[*] Verifying John Doe with Person 2's face (p2a)...")
    print_result(v_mismatch_txn, tester.wait_for_callback(v_mismatch_txn))

    # ---------------------------------------------------------
    print_phase("4. SEMANTIC NAME MATCHING (BERT)")
    # ---------------------------------------------------------
    v_semantic_txn = tester.verify(
        get_image_path("p1b.png"), "Johnathan Doe", "1990-01-01", "MALE"
    )
    print("[*] Verifying 'Johnathan Doe' (p1b) against enrolled 'John Doe'...")
    print_result(v_semantic_txn, tester.wait_for_callback(v_semantic_txn))

    # ---------------------------------------------------------
    print_phase("5. NAME MISMATCH (IDENTITY SANITY CHECK)")
    # ---------------------------------------------------------
    v_veto_txn = tester.verify(
        get_image_path("p1b.png"), "Zuck Musk", "1990-01-01", "MALE"
    )
    print("[*] Verifying 'Zuck Musk' (p1b) against enrolled 'John Doe'...")
    print_result(v_veto_txn, tester.wait_for_callback(v_veto_txn))

    # ---------------------------------------------------------
    print_phase("6. DEMOGRAPHIC VARIATION")
    # ---------------------------------------------------------
    v_demo_txn = tester.verify(
        get_image_path("p1b.png"), "John Doe", "1980-01-01", "MALE"
    )
    print("[*] Verifying John Doe (p1b) with wrong DOB (1980 vs 1990)...")
    print_result(v_demo_txn, tester.wait_for_callback(v_demo_txn))

    # ---------------------------------------------------------
    print_phase("7. SEARCH & RETRIEVAL")
    # ---------------------------------------------------------
    print("[*] Searching for 'John Doe'...")
    results = tester.search(name="John Doe")
    print(f"  [+] Found {len(results)} records")
    for r in results:
        print(f"      - ID: {r['transaction_id']}, Name: {r['name']}")

    # ---------------------------------------------------------
    print_phase("8. NICKNAME TEST (BERT POWER)")
    # ---------------------------------------------------------
    v_nick_txn = tester.verify(
        get_image_path("p3b.png"), "Ali Smith", "1985-05-20", "FEMALE"
    )
    print("[*] Verifying 'Ali Smith' (p3b) against enrolled 'Alice Smith'...")
    print_result(v_nick_txn, tester.wait_for_callback(v_nick_txn))

    # ---------------------------------------------------------
    print_phase("9. TWINS CHECK (BIOMETRIC SIMILARITY)")
    # ---------------------------------------------------------
    # Enrolling first twin
    t1a_txn = tester.enroll(
        get_image_path("twin3a.png"), "James Twin", "1995-10-10", "MALE"
    )
    print("[*] Enrolling Twin A (James Twin)...")
    cb_t1a = tester.wait_for_callback(t1a_txn)
    print_result(t1a_txn, cb_t1a)

    # Verifying second twin against first twin's identity
    t1b_txn = tester.verify(
        get_image_path("twin3b.png"), "James Twin", "1995-10-10", "MALE"
    )
    print(
        "[*] Verifying Twin B against James Twin's identity (High Face Similarity)..."
    )
    print_result(t1b_txn, tester.wait_for_callback(t1b_txn))

    print("\n" + "=" * 60)
    print(" ALL PHASES COMPLETED")
    print("=" * 60)


if __name__ == "__main__":
    run_suite()
