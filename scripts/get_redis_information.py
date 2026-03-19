import redis
import json
import time
import random
import sys


if __name__ == "__main__":
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    slug_keys = r.keys("slug:assets:*")
    tracked_slugs = [keys.replace("slug:assets:", "") for keys in slug_keys]
    for slug in tracked_slugs:
        print(f"Slug name : {slug}")
        token_ids = r.smembers(f"slug:assets:{slug}")
        for token_id in token_ids:
            print(f"Token ID : {token_id}")
            token_info = r.hgetall(f"token:meta:{token_id}")
            print(f"Token Info : {token_info}")

    portfolio = r.hgetall("portfolio:balance")

    print(portfolio)
