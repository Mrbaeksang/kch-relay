import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from verify_contract import Verifier, validate_cookie


class Reference(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('X-Request-ID', 'reference')
        self.end_headers()
        body = {'data': {'allowed': True}}
        if self.path.endswith('/bad-shape'):
            body = {'looksFine': True}
        if self.path.endswith('/bad-meta'):
            body['meta'] = {'hasUnreadMessages': 'false', 'hasUnreadNotifications': False}
        self.wfile.write(json.dumps(body).encode())

    def log_message(self, *_):
        pass


class ContractValidatorTest(unittest.TestCase):
    def test_reference_http_fixtures_detect_invalid_results(self):
        server = HTTPServer(('127.0.0.1', 0), Reference)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            check = Verifier(f'http://127.0.0.1:{server.server_port}', 'http://reference.invalid')
            check.call('GET', '/correct')
            with self.assertRaises(AssertionError):
                check.call('GET', '/bad-shape')
            with self.assertRaises(AssertionError):
                check.call('GET', '/bad-meta')
            with self.assertRaises(AssertionError):
                check.call('GET', '/admin-missing-permission', 403)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_missing_cookie_protection_is_detected(self):
        with self.assertRaises(AssertionError):
            validate_cookie('relay_session=value; Path=/api')
        validate_cookie('relay_session=value; Path=/api; HttpOnly; SameSite=Lax')
