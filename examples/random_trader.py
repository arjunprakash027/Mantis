import redis
import json
import time
import random
import sys

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

def random_trader(token_id):
    """Simulates a basic trader that places random small BUY/SELL orders."""
    print(f"🎲 Random Trader started for token {token_id}")
    
    actions = ["BUY", "SELL"]

    try:
        while True:
            action = random.choice(actions)
            amount = round(random.uniform(0.1, 5.0), 2)
            
            signal = {
                "action": action,
                "asset": token_id,
                "amount": amount,
                "strategy_id": "random_luck_v1"
            }
            
            r.xadd("signals:inbound", {"data": json.dumps(signal)})
            print(f"🚀 Placed {action} for {amount} units")
            
            time.sleep(random.randint(5, 15))
            
    except KeyboardInterrupt:
        print("\nStopping trader...")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python random_trader.py <TOKEN_ID>")
    else:
        random_trader(sys.argv[1])
