#!/bin/bash

echo "=== Logging in ==="
AUTH_RESP=$(curl -s -X POST http://localhost:8080/auth/login -H "Content-Type: application/json" -d '{"username": "admin", "password": "password"}')
TOKEN=$(echo $AUTH_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['access_token'])")

echo -e "\n=== 1. Enrolling 'Jane Doe' ==="
ENROLL_RESP=$(curl -s -X POST http://localhost:8080/kyc/enroll \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
    "name": "Jane Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')
TXN_ID=$(echo $ENROLL_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")
sleep 3

echo -e "\n=== 2. Verifying 'Janet Doe' (Different Name) ==="
VERIFY_RESP=$(curl -s -X POST http://localhost:8080/kyc/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
    "name": "Janet Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')
VERIFY_TXN_ID=$(echo $VERIFY_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")
sleep 5

echo -e "\n=== 3. Checking Verification Result for 'Janet Doe' ==="
curl -s -X GET "http://localhost:8080/kyc/status/$VERIFY_TXN_ID" -H "Authorization: Bearer $TOKEN"
