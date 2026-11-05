#!/usr/bin/env python3
"""Verify the actual foundation HTTP contract; no production fixture endpoints."""
import argparse
import http.cookiejar
import json
import urllib.error
import urllib.request


def expect(actual, expected, message):
    if actual != expected:
        raise AssertionError(f'{message}: expected {expected}, received {actual}')


def validate_envelope(status, body, headers):
    if not isinstance(body, dict):
        raise AssertionError('JSON object required')
    request_id = headers.get('X-Request-ID')
    if not request_id:
        raise AssertionError('request ID missing')
    if status >= 400:
        error = body.get('error', {})
        if not error.get('code') or not error.get('message') or error.get('requestId') != request_id:
            raise AssertionError('standard error contract missing')
    elif 'data' not in body:
        raise AssertionError('success data missing')
    if 'meta' in body:
        for key in ('hasUnreadMessages', 'hasUnreadNotifications'):
            if type(body['meta'].get(key)) is not bool:
                raise AssertionError('unread metadata must contain booleans')


def validate_cookie(header):
    if not header or 'relay_session=' not in header or 'HttpOnly' not in header or 'Path=/api' not in header or 'SameSite=Lax' not in header:
        raise AssertionError('session cookie contract missing')


def validate_user(data, account, role):
    user = data['user']
    expect(set(user), {'id', 'realName', 'department', 'jobLevel', 'role', 'status'}, 'identity fields')
    expect(user['id'], account, 'selected identity')
    expect(user['role'], role, 'server role')
    expect(user['status'], 'ACTIVE', 'user status')
    if len(data.get('csrfToken', '')) < 43:
        raise AssertionError('CSRF token missing')


class Verifier:
    def __init__(self, base, origin, cookie_file=None):
        self.base = base.rstrip('/')
        self.origin = origin
        self.jar = http.cookiejar.LWPCookieJar(cookie_file) if cookie_file else http.cookiejar.CookieJar()
        if cookie_file:
            from pathlib import Path
            if Path(cookie_file).exists():
                self.jar.load(ignore_discard=True, ignore_expires=True)
        self.client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))

    def call(self, method, path, expected=200, data=None, csrf=None, cookie=None, origin=None):
        headers = {'Accept': 'application/json', 'Origin': self.origin if origin is None else origin}
        body = None
        if data is not None:
            headers['Content-Type'] = 'application/json'
            body = json.dumps(data).encode()
        if csrf is not None:
            headers['X-CSRF-Token'] = csrf
        if cookie is not None:
            headers['Cookie'] = cookie
        request = urllib.request.Request(self.base + '/api/v1' + path, data=body, headers=headers, method=method)
        try:
            response = self.client.open(request, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            status = response.status
            raw = response.read()
            result = json.loads(raw)
            validate_envelope(status, result, response.headers)
            expect(status, expected, method + ' ' + path)
            return result, response.headers

    def session_cookie(self):
        return next(c.name + '=' + c.value for c in self.jar if c.name == 'relay_session')

    def run(self, mode):
        config, _ = self.call('GET', '/auth/config')
        self.call('GET', '/auth/me', 401)
        if mode == 'disabled':
            expect(config['data']['developmentLogin'], False, 'disabled auth')
            expect(config['data']['accounts'], [], 'disabled accounts')
            self.call('POST', '/auth/development-login', 404, {'accountKey': 'dev-admin'})
            self.call('GET', '/admin/access', 401)
            self.call('GET', '/auth/me', 401, cookie='relay_session=' + 'a' * 64)
            print('Actual disabled runtime: no development login or authentication bypass')
            return
        expect(config['data']['developmentLogin'], True, 'development auth')
        expect({x['key'] for x in config['data']['accounts']}, {'dev-admin', 'dev-employee-a', 'dev-employee-b'}, 'account choices')
        self.call('POST', '/auth/development-login', 403, {'accountKey': 'dev-admin'}, origin='http://attacker.invalid')
        self.call('POST', '/auth/development-login', 400, {'accountKey': 'arbitrary'})
        self.call('POST', '/auth/development-login', 400, {'accountKey': 'dev-admin', 'role': 'HR_ADMIN'})
        admin, headers = self.call('POST', '/auth/development-login', data={'accountKey': 'dev-admin'})
        validate_cookie(headers.get('Set-Cookie'))
        validate_user(admin['data'], 'dev-admin', 'HR_ADMIN')
        old_cookie = self.session_cookie()
        self.call('GET', '/admin/access')
        self.call('GET', '/auth/me')
        self.call('POST', '/auth/logout', 403, {}, csrf='invalid')
        for key in ('dev-employee-a', 'dev-employee-b'):
            session, _ = self.call('POST', '/auth/development-login', data={'accountKey': key})
            validate_user(session['data'], key, 'MEMBER')
            self.call('GET', '/access')
            self.call('GET', '/admin/access', 403)
        self.call('GET', '/auth/me', 401, cookie=old_cookie)
        session, _ = self.call('GET', '/auth/me')
        active_cookie = self.session_cookie()
        self.call('GET', '/not-implemented', 404)
        self.call('POST', '/auth/logout', data={}, csrf=session['data']['csrfToken'])
        self.call('GET', '/auth/me', 401)
        self.call('GET', '/auth/me', 401, cookie=active_cookie)
        print('Actual development runtime: accounts, sessions, role checks, CSRF, revocation, and standard errors OK')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--base-url', required=True)
    parser.add_argument('--origin')
    parser.add_argument('--cookie-file')
    parser.add_argument('--session-probe', choices=('create', 'resume'))
    parser.add_argument('--mode', choices=('development', 'disabled'), required=True)
    args = parser.parse_args()
    check = Verifier(args.base_url, args.origin or args.base_url.rstrip('/'), args.cookie_file)
    if args.session_probe:
        if not args.cookie_file:
            parser.error('--session-probe requires --cookie-file')
        if args.session_probe == 'create':
            check.call('POST', '/auth/development-login', data={'accountKey': 'dev-employee-a'})
            check.jar.save(ignore_discard=True, ignore_expires=True)
            from pathlib import Path
            Path(args.cookie_file).chmod(0o600)
        session, _ = check.call('GET', '/auth/me')
        validate_user(session['data'], 'dev-employee-a', 'MEMBER')
        print('Actual persisted session probe:', args.session_probe, 'OK')
    else:
        check.run(args.mode)
