#!/usr/bin/env python3
"""Measure a fresh synthetic registry and independently check local recovery."""
import argparse
import hashlib
import http.cookiejar
import json
import platform
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--web-dir', required=True)
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    binary, web = str(Path(args.binary).resolve()), str(Path(args.web_dir).resolve())

    def command(*items):
        result = subprocess.run([binary, *items], check=True, capture_output=True, text=True)
        return json.loads(result.stdout)

    with tempfile.TemporaryDirectory(prefix='society-baseline-') as directory:
        root = Path(directory)
        db, bundle, restored = root / 'live' / 'society.db', root / 'snapshot', root / 'restored' / 'society.db'
        seeded = command('seed-demo', '--demo', '--db', str(db))
        assert seeded['counts']['flats'] == 118
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        base = f'http://127.0.0.1:{port}'
        with (root / 'server.log').open('w') as logs:
            process = subprocess.Popen([binary, 'serve', '--demo', '--db', str(db), '--mfa-key-file', str(root / 'keys' / 'mfa.key'), '--addr', f'127.0.0.1:{port}', '--web-dir', web], stdout=logs, stderr=logs)
            try:
                deadline = time.monotonic() + 15
                while True:
                    try:
                        with urllib.request.urlopen(base + '/ready', timeout=1) as response:
                            assert json.load(response)['status'] == 'ready'
                        break
                    except (urllib.error.URLError, TimeoutError):
                        if process.poll() is not None or time.monotonic() >= deadline:
                            raise RuntimeError('Baseline server did not become ready')
                        time.sleep(0.05)

                client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
                login_start = time.perf_counter_ns()
                login_request = urllib.request.Request(base + '/api/auth/login', data=json.dumps({'login': 'admin@demo.society', 'password': 'Community-preview-2026!'}).encode(), headers={'Origin': base, 'Content-Type': 'application/json'}, method='POST')
                with client.open(login_request, timeout=5) as response:
                    identity = json.load(response)
                    assert identity['mfa_pending'] is True and identity['can_manage_registry'] is False
                login_ms = (time.perf_counter_ns() - login_start) / 1_000_000

                def write(path, body):
                    req = urllib.request.Request(base + path, data=json.dumps(body).encode(), headers={'Origin': base, 'Content-Type': 'application/json', 'X-CSRF-Token': identity['csrf_token']}, method='POST')
                    with client.open(req, timeout=5) as response:
                        return json.load(response)

                mfa_start = time.perf_counter_ns()
                write('/api/auth/mfa/setup', {})
                preview = write('/api/auth/mfa/demo-code', {})
                verified = write('/api/auth/mfa/confirm', {'code': preview['code']})
                assert verified['user']['can_manage_registry'] and len(verified['recovery_codes']) == 10
                mfa_ms = (time.perf_counter_ns() - mfa_start) / 1_000_000

                def read(path):
                    with client.open(base + path, timeout=5) as response:
                        return json.load(response)

                system = read('/api/system')
                samples = []
                for i in range(110):
                    start = time.perf_counter_ns()
                    page = read('/api/flats?page=1&page_size=12')
                    elapsed = (time.perf_counter_ns() - start) / 1_000_000
                    assert page['total'] == 118 and len(page['items']) == 12
                    if i >= 10:
                        samples.append(elapsed)
                manifest = command('snapshot', '--db', str(db), '--out', str(bundle))
                snapshot_hash = hashlib.sha256((bundle / 'society.db').read_bytes()).hexdigest()
                assert manifest['sha256'] == snapshot_hash
                recovery = command('restore-check', '--snapshot', str(bundle), '--out', str(restored))
                assert hashlib.sha256(restored.read_bytes()).hexdigest() == snapshot_hash
                assert recovery['manifest']['counts'] == seeded['counts']
                restored_system = command('inspect', '--db', str(restored))
                assert restored_system['counts'] == seeded['counts']
                assert restored_system['engine']['journal_mode'] == 'wal'

                ordered = sorted(samples)
                def percentile(p):
                    return round(ordered[min(len(ordered) - 1, int((len(ordered) - 1) * p))], 3)

                report = {
                    'recorded_at': datetime.now(timezone.utc).isoformat(),
                    'environment': {
                        'os': platform.system(), 'architecture': platform.machine(),
                        'go': subprocess.check_output(['go', 'version'], text=True).strip(),
                        'node': subprocess.check_output(['node', '--version'], text=True).strip(),
                    },
                    'system': system,
                    'fixture': seeded,
                    'authentication': {'role': 'demo registry officer', 'login_ms': round(login_ms, 3), 'mfa_enrollment_ms': round(mfa_ms, 3), 'mfa': 'TOTP plus single-use recovery codes; synthetic preview helper used for this run', 'session_policy': '30 min idle / 8 hr absolute', 'password': 'Argon2id 64 MiB / 3 passes / 2 lanes'},
                    'registry_read': {
                        'path': '/api/flats?page=1&page_size=12', 'transport': 'local HTTP',
                        'concurrent_clients': 1, 'warmup_requests': 10, 'sample_count': len(samples),
                        'p50_ms': percentile(.50), 'p75_ms': percentile(.75), 'p95_ms': percentile(.95),
                        'max_ms': round(max(samples), 3), 'samples_ms': samples,
                    },
                    'recovery': {
                        'snapshot_bytes': manifest['bytes'], 'sha256': snapshot_hash,
                        'restore_elapsed_ms': recovery['elapsed_ms'],
                        'counts': recovery['manifest']['counts'],
                        'independent_checksum_match': True,
                        'restored_connection_settings': restored_system['engine'],
                    },
                    'limitations': [
                        'Synthetic local development baseline; no production SLA.',
                        'Single sequential client; concurrent hardware workload is pending.',
                        'Local unencrypted snapshots only; off-site encryption/custodian recovery is pending.',
                        'Manual entries, receipts, real data migration and production identity/custody acceptance are pending.',
                    ],
                }
            finally:
                process.terminate()
                try:
                    process.wait(timeout=6)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'report': str(output), 'registry_read_p95_ms': report['registry_read']['p95_ms'], 'restore_ms': report['recovery']['restore_elapsed_ms'], 'counts': report['fixture']['counts']}, indent=2))


if __name__ == '__main__':
    main()
