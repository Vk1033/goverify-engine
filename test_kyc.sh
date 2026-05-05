#!/bin/bash
echo "=== Testing KYC Enrollment ==="
ENROLL_RESP=$(curl -s -X POST http://localhost:8080/kyc/enroll \
  -H "Authorization: Bearer my-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "dummy_photo_data_here",
    "name": "Jane Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')

echo $ENROLL_RESP

TXN_ID=$(echo $ENROLL_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")

echo -e "\n=== Waiting for async processing ==="
sleep 2

echo -e "\n=== Checking Status for $TXN_ID ==="
curl -s -X GET "http://localhost:8080/kyc/status/$TXN_ID" \
  -H "Authorization: Bearer my-test-token" 

echo -e "\n\n=== Testing KYC Verification ==="
VERIFY_RESP=$(curl -s -X POST http://localhost:8080/kyc/verify \
  -H "Authorization: Bearer my-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "photo_base64": "dummy_photo_data_here",
    "name": "Jane Doe",
    "dob": "1990-01-01",
    "gender": "FEMALE"
}')

echo $VERIFY_RESP

VERIFY_TXN_ID=$(echo $VERIFY_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")

echo -e "\n=== Waiting for async verification processing ==="
sleep 2

echo -e "\n=== Checking Status for $VERIFY_TXN_ID ==="
curl -s -X GET "http://localhost:8080/kyc/status/$VERIFY_TXN_ID" \
  -H "Authorization: Bearer my-test-token"
