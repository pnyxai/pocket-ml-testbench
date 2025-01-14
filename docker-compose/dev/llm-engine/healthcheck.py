import requests

response = requests.get('http://localhost:9087/health')

if response.status_code == 200:
    exit(0)
else:
    exit(1)