import os
import urllib.request


target = os.environ.get("AUTOCURL_DEMO_URL", "https://example.com")
with urllib.request.urlopen(target, timeout=10) as response:
    print(response.status)
