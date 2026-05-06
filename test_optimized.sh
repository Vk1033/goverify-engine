#!/bin/bash

echo "=== Logging in to get token ==="
AUTH_RESP=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}')

TOKEN=$(echo $AUTH_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['access_token'])")

if [ -z "$TOKEN" ]; then
    echo "Failed to get token: $AUTH_RESP"
    exit 1
fi

echo "Token obtained: ${TOKEN:0:10}..."

echo -e "\n=== Testing KYC Enrollment ==="
ENROLL_RESP=$(curl -s -X POST http://localhost:8080/kyc/enroll \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
    "name": "Jane Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')

echo "Enrollment Response: $ENROLL_RESP"

TXN_ID=$(echo $ENROLL_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")

echo -e "\n=== Waiting for async processing (5s) ==="
sleep 5

echo -e "\n=== Checking Status for $TXN_ID ==="
STATUS_RESP=$(curl -s -X GET "http://localhost:8080/kyc/status/$TXN_ID" \
  -H "Authorization: Bearer $TOKEN")
echo "Status Response: $STATUS_RESP"

echo -e "\n=== Testing KYC Verification ==="
VERIFY_RESP=$(curl -s -X POST http://localhost:8080/kyc/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
    "name": "Jane Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')

echo "Verification Response: $VERIFY_RESP"

VERIFY_TXN_ID=$(echo $VERIFY_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")

echo -e "\n=== Waiting for async verification processing (5s) ==="
sleep 5

echo -e "\n=== Checking Status for $VERIFY_TXN_ID ==="
VERIFY_STATUS_RESP=$(curl -s -X GET "http://localhost:8080/kyc/status/$VERIFY_TXN_ID" \
  -H "Authorization: Bearer $TOKEN")
echo "Verification Status: $VERIFY_STATUS_RESP"
