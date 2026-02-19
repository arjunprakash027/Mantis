import redis
import json
import time
import sys

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

def stream_orderbook(token_id):
    """Simple listener for orderbook updates from Mantis."""
    stream_key = f"orderbook:stream:{token_id}"
    print(f"👂 Listening for updates on {stream_key}...")
    
    last_id = '$'
    
    try:
        while True:
            resp = r.xread({stream_key: last_id}, block=0)
            if resp:
                for _, messages in resp:
                    for msg_id, data in messages:
                        raw_payload = data['data']
                        payload = json.loads(raw_payload)
                        
                        bid = payload.get('best_bid', 'N/A')
                        ask = payload.get('best_ask', 'N/A')
                        
                        print(f"[{time.strftime('%H:%M:%S')}] Bid: {bid} | Ask: {ask}")
                        last_id = msg_id
    except KeyboardInterrupt:
        print("\nStopping...")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python get_order_book.py <TOKEN_ID>")
    else:
        stream_orderbook(sys.argv[1])
