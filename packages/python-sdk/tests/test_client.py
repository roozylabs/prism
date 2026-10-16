import os
import sys
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../src")))

from prism.client import Prism, AsyncPrism

class TestPrismClient(unittest.TestCase):
    def test_client_init(self):
        client = Prism(api_key="prism_sk_test", base_url="https://api.prism.roozylabs.com")
        self.assertEqual(client.api_key, "prism_sk_test")
        self.assertEqual(client.base_url, "https://api.prism.roozylabs.com")

    def test_headers_serialization(self):
        client = Prism(api_key="prism_sk_test")
        headers = client._headers(agent_id="test-agent")
        self.assertEqual(headers["Authorization"], "Bearer prism_sk_test")
        self.assertEqual(headers["X-Prism-Agent-ID"], "test-agent")
        self.assertEqual(headers["User-Agent"], "roozylabs-prism-python/2.1.0")

if __name__ == "__main__":
    unittest.main()
