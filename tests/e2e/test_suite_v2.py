import os
import base64
import time
import requests
import json
from concurrent.futures import ThreadPoolExecutor
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading

# Configuration
API_URL = os.getenv("API_URL", "http://localhost:8080")
CALLBACK_PORT = 9999
CALLBACK_HOST = os.getenv("CALLBACK_HOST", "host.docker.internal")
CALLBACK_URL = f"http://{CALLBACK_HOST}:{CALLBACK_PORT}/callback"

# Global store for callbacks
callbacks = {}


class CallbackHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers["Content-Length"])
        post_data = self.rfile.read(content_length)
        data = json.loads(post_data)
        txn_id = data.get("transaction_id")
        if txn_id:
            callbacks[txn_id] = data
        self.send_response(200)
        self.end_headers()

    def log_message(self, format, *args):
        return  # Silent


def start_callback_server():
    server = HTTPServer(("0.0.0.0", CALLBACK_PORT), CallbackHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


class KYCTester:
    def __init__(self):
        self.token = None

    def register(self, username="admin", password="password123"):
        resp = requests.post(
            f"{API_URL}/auth/register", json={"username": username, "password": password}
        )
        if resp.status_code in [201, 400]:
            return True
        return False

    def login(self, username="admin", password="password123"):
        self.register(username, password)
        resp = requests.post(
            f"{API_URL}/auth/login", json={"username": username, "password": password}
        )
        if resp.status_code == 200:
            self.token = resp.json().get("access_token")
            return True
        return False

    def get_headers(self):
        return {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}

    def enroll(self, image_path, name, dob, gender):
        with open(image_path, "rb") as f:
            img_b64 = base64.b64encode(f.read()).decode()

        payload = {
            "photo_base64": img_b64,
            "name": name,
            "dob": dob,
            "gender": gender,
            "callback_url": CALLBACK_URL,
        }
        resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=self.get_headers())
        return resp.json().get("transaction_id")

    def verify(self, image_path, name, dob, gender):
        with open(image_path, "rb") as f:
            img_b64 = base64.b64encode(f.read()).decode()

        payload = {
            "photo_base64": img_b64,
            "name": name,
            "dob": dob,
            "gender": gender,
            "callback_url": CALLBACK_URL,
        }
        resp = requests.post(f"{API_URL}/kyc/verify", json=payload, headers=self.get_headers())
        return resp.json().get("transaction_id")

    def wait_for_callback(self, txn_id, timeout=60):
        start = time.time()
        while time.time() - start < timeout:
            if txn_id in callbacks:
                return callbacks.pop(txn_id)
            time.sleep(0.5)
        return None


def get_image_path(name):
    return os.path.join("testdata", "images", name)


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
    start_callback_server()

    if not tester.login():
        print("[!] Login failed. Is the API running?")
        return

    # ---------------------------------------------------------
    print_phase("1. CONCURRENT MASS ENROLLMENT")
    # ---------------------------------------------------------
    subjects = [
        ("p1a.png", "John Doe", "1990-01-01", "MALE"),
        ("p3a.png", "Alice Smith", "1985-05-20", "FEMALE"),
        ("twin1a.png", "James Twin", "2000-01-01", "MALE"),
        ("twin2a.jpeg", "Sarah Twin", "1995-10-10", "FEMALE"),
        ("twin3a.png", "Michael Twin", "1988-12-12", "MALE"),
        ("c1a.jpg", "HighRes Subject", "1992-03-15", "MALE"),
    ]

    def do_enroll(data):
        img, name, dob, gender = data
        print(f"[*] Dispatching enrollment: {name}...")
        txn = tester.enroll(get_image_path(img), name, dob, gender)
        cb = tester.wait_for_callback(txn)
        return txn, cb, name

    with ThreadPoolExecutor(max_workers=len(subjects)) as executor:
        enroll_results = list(executor.map(do_enroll, subjects))

    for txn, cb, name in enroll_results:
        print(f"\n[+] Results for {name}:")
        print_result(txn, cb)

    # ---------------------------------------------------------
    print_phase("2. CONCURRENT BIOMETRIC VERIFICATION")
    # ---------------------------------------------------------
    verifications = [
        ("p1b.png", "John Doe", "1990-01-01", "MALE", "John Match"),
        ("p3b.png", "Alice Smith", "1985-05-20", "FEMALE", "Alice Match"),
        ("c1b.jpg", "HighRes Subject", "1992-03-15", "MALE", "HighRes Match"),
        ("p2a.png", "John Doe", "1990-01-01", "MALE", "Face Mismatch (Veto)"),
        ("p1b.png", "Johnathan Doe", "1990-01-01", "MALE", "Semantic Match"),
        ("twin1b.png", "James Twin", "2000-01-01", "MALE", "Twin Set 1"),
        ("twin2b.jpeg", "Sarah Twin", "1995-10-10", "FEMALE", "Twin Set 2"),
        ("twin3b.png", "Michael Twin", "1988-12-12", "MALE", "Twin Set 3"),
        ("p1b.png", "Imposter Bob", "1970-01-01", "MALE", "Fraud Attempt"),
    ]

    def do_verify(data):
        img, name, dob, gender, label = data
        print(f"[*] Dispatching verification: {label}...")
        txn = tester.verify(get_image_path(img), name, dob, gender)
        cb = tester.wait_for_callback(txn)
        return txn, cb, label

    with ThreadPoolExecutor(max_workers=5) as executor: # Throttle a bit to test queueing
        verify_results = list(executor.map(do_verify, verifications))

    for txn, cb, label in verify_results:
        print(f"\n[+] Results for {label}:")
        print_result(txn, cb)


if __name__ == "__main__":
    run_suite()
