import time
import json
import test_with_images
from test_with_images import login, enroll, verify, get_status

def test():
    print("Authenticating...")
    token = login("admin", "password123")
    test_with_images.TOKEN = token
    
    print("Enrolling person1_a.png...")
    name, dob, gender = "Test Person", "1990-01-01", "FEMALE"
    try:
        enroll_resp = enroll("person1_a.png", name, dob, gender)
    except Exception as e:
        print(f"Enrollment request failed: {e}")
        return
        
    txn_id = enroll_resp.get("transaction_id")
    if not txn_id:
        print(f"Failed to get transaction_id from enroll response: {enroll_resp}")
        return
        
    print(f"Enroll txn: {txn_id}. Polling status...")
    status = {}
    for _ in range(15):
        try:
            status = get_status(txn_id)
            print(f"Current enroll status: {status.get('status')}")
            if status.get("status") in ["SUCCESS", "FAILED"]:
                break
        except Exception as e:
            print(f"Status check failed: {e}")
        time.sleep(2)
        
    print(f"Final Enroll status: {status}")
    if status.get("status") != "SUCCESS":
        print("Enrollment did not succeed. Aborting verification.")
        return
        
    print("\nVerifying person1_b.png...")
    try:
        verify_resp = verify("person1_b.png", name, dob, gender)
    except Exception as e:
        print(f"Verify request failed: {e}")
        return
        
    v_txn_id = verify_resp.get("transaction_id")
    if not v_txn_id:
        print(f"Failed to get transaction_id from verify response: {verify_resp}")
        return
        
    print(f"Verify txn: {v_txn_id}. Polling status...")
    v_status = {}
    for _ in range(15):
        try:
            v_status = get_status(v_txn_id)
            print(f"Current verify status: {v_status.get('status')}")
            if v_status.get("status") in ["SUCCESS", "FAILED", "PARTIAL_MATCH", "NO_MATCH"]:
                break
        except Exception as e:
            print(f"Status check failed: {e}")
        time.sleep(2)
        
    print("\n--------------------------")
    print(f"Final Verify result: {v_status.get('status')}")
    print(f"Confidence score: {v_status.get('confidence_score')}")
    print(f"Details: {json.dumps(v_status.get('details', {}), indent=2)}")

if __name__ == "__main__":
    test()
