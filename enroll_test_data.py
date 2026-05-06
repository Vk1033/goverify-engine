import requests
import base64
import time

API_URL = "http://localhost:8080"

def get_image_base64(path):
    with open(path, "rb") as image_file:
        return base64.b64encode(image_file.read()).decode('utf-8')

def authenticate():
    resp = requests.post(f"{API_URL}/auth/login", json={"username": "admin", "password": "password123"})
    return resp.json()["access_token"]

def enroll(token, name, photo_path):
    print(f"--- Enrolling {name} ---")
    payload = {
        "name": name,
        "photo_base64": get_image_base64(photo_path),
        "dob": "1980-05-20",
        "gender": "MALE"
    }
    headers = {"Authorization": f"Bearer {token}"}
    resp = requests.post(f"{API_URL}/kyc/enroll", json=payload, headers=headers)
    print(resp.json())

def main():
    token = authenticate()
    enroll(token, "John Doe", "person1_a.png")
    print("\nWaiting 5 seconds for enrollment to process...")
    time.sleep(5)

if __name__ == "__main__":
    main()
