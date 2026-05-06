#!/bin/bash

# Encode images to base64
P1A_B64=$(base64 -w 0 p1a.png)
P1B_B64=$(base64 -w 0 p1b.png)

echo "=== Logging in ==="
AUTH_RESP=$(curl -s -X POST http://localhost:8080/auth/login -H "Content-Type: application/json" -d '{"username": "admin", "password": "password"}')
TOKEN=$(echo $AUTH_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['access_token'])")

echo -e "\n=== 1. Enrolling Person 1 (using p1a.png) ==="
cat <<EOF > enroll_req.json
{
    "photo_base64": "$P1A_B64",
    "name": "John Smith",
    "dob": "1985-05-15",
    "gender": "MALE"
}
EOF

ENROLL_RESP=$(curl -s -X POST http://localhost:8080/kyc/enroll \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @enroll_req.json)
TXN_ID=$(echo $ENROLL_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")
echo "Enrollment TXN: $TXN_ID"

echo -e "\n=== Waiting for async enrollment (5s) ==="
sleep 5

echo -e "\n=== 2. Verifying Person 1 (using p1b.png - different photo) ==="
cat <<EOF > verify_req.json
{
    "photo_base64": "$P1B_B64",
    "name": "John Smith",
    "dob": "1985-05-15",
    "gender": "MALE"
}
EOF

VERIFY_RESP=$(curl -s -X POST http://localhost:8080/kyc/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @verify_req.json)
VERIFY_TXN_ID=$(echo $VERIFY_RESP | python3 -c "import sys, json; print(json.load(sys.stdin)['transaction_id'])")
echo "Verification TXN: $VERIFY_TXN_ID"

echo -e "\n=== Waiting for async verification (5s) ==="
sleep 5

echo -e "\n=== 3. Final Result for Real Image Verification ==="
curl -s -X GET "http://localhost:8080/kyc/status/$VERIFY_TXN_ID" -H "Authorization: Bearer $TOKEN"

# Cleanup
rm enroll_req.json verify_req.json
