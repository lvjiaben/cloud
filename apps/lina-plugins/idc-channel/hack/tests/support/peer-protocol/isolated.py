"""TC011 owns both local databases, processes, packages and TLS fixtures.

Official init creates host schema/seed; normal source-plugin startup creates
plugin schema/required seed. No deployment metadata or business data is cloned.
Every SQL write and DROP targets a randomly named database owned by this test.
"""
from contextlib import ExitStack
from pathlib import Path
from urllib.parse import urlparse, unquote, quote
import base64
import copy
import hashlib
import http.client
import http.server
import ipaddress
import json
import os
import secrets
import shutil
import signal
import socket
import ssl
import subprocess
import threading
import time
import urllib.error
import urllib.request
import yaml

ROOT = Path(__file__).resolve().parents[7]
PREFIX = '/x/idc-core/api/v1/admin'

class APIError(AssertionError):
    def __init__(self,path,method,envelope):
        self.error_code=envelope.get("errorCode","")
        super().__init__(f'Public API rejected {method} {path.split("?")[0]} code={envelope.get("code")}')


class API:
    def __init__(self, port):
        self.origin = f'http://127.0.0.1:{port}'
        self.authorization = ''
        token = self.call('/api/v1/auth/login', 'POST', {
            'username': os.getenv('E2E_ADMIN_USER', 'admin'),
            'password': os.getenv('E2E_ADMIN_PASS', 'admin123'), 'clientType': 'web',
        })['accessToken']
        self.authorization = 'Bearer ' + token

    def call(self, path, method='GET', body=None):
        request = urllib.request.Request(self.origin + path, method=method,
            data=None if body is None else json.dumps(body).encode(),
            headers={'Content-Type': 'application/json', 'Authorization': self.authorization})
        try:
            response = urllib.request.urlopen(request, timeout=120)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            envelope = json.load(response)
        if envelope.get('code') != 0:
            # Response messages can contain provider data. Report only the fixed path.
            raise APIError(path,method,envelope)
        return envelope.get('data')

    def channel(self, credential):
        result = copy.copy(self)
        result.authorization = 'Basic ' + base64.b64encode(
            (credential['appId'] + ':' + credential['token']).encode()).decode()
        return result

    def upload(self, package):
        boundary = 'tc011-' + secrets.token_hex(24)
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{package.name}"\r\n'
                'Content-Type: application/wasm\r\n\r\n').encode() + package.read_bytes()
        body += f'\r\n--{boundary}--\r\n'.encode()
        request = urllib.request.Request(self.origin + '/api/v1/plugins/dynamic/package',
            data=body, headers={'Authorization': self.authorization,
            'Content-Type': 'multipart/form-data; boundary=' + boundary}, method='POST')
        with urllib.request.urlopen(request, timeout=120) as response:
            envelope = json.load(response)
        assert envelope.get('code') == 0, 'Formal plugin upload failed'

class Instances:

    def __init__(self):
        self.stack = ExitStack()
        self.databases = []
        self.environment = dict(os.environ)
        if os.getenv('E2E_PG_URL'):
            u = urlparse(os.environ['E2E_PG_URL'])
            self.environment.update(PGHOST=u.hostname, PGPORT=str(u.port or 5432),
                PGUSER=unquote(u.username or 'postgres'), PGPASSWORD=unquote(u.password or ''),
                PGDATABASE=u.path.lstrip('/'), PGSSLMODE='disable')
        else:
            for pg, e2e, default in [('PGHOST','E2E_DB_HOST','127.0.0.1'),
                ('PGPORT','E2E_DB_PORT','5432'),('PGUSER','E2E_DB_USER','postgres'),
                ('PGPASSWORD','E2E_DB_PASSWORD','postgres'),('PGDATABASE','E2E_DB_NAME','linapro'),
                ('PGSSLMODE','E2E_DB_SSLMODE','disable')]:
                self.environment[pg] = os.getenv(e2e, default)
        assert self.environment['PGHOST'] in ['127.0.0.1', 'localhost', '::1'], 'TC011 requires local PostgreSQL'
        self.name = 'linapro_test_peer_' + secrets.token_hex(8)
        date = time.strftime('%Y%m%d')
        self.directory = ROOT / 'temp' / date / (time.strftime('%H%M%S') + '-' + self.name)
        self.public_ip = os.getenv('E2E_PEER_PUBLIC_IP', '')
        if not self.public_ip:
            output = self.run(['ip','-j','address','show'], capture=True)
            addresses = [a['local'] for link in json.loads(output) for a in link.get('addr_info', [])]
            self.public_ip = next((a for a in addresses if ipaddress.ip_address(a).is_global), '')
        assert self.public_ip and ipaddress.ip_address(self.public_ip).is_global, 'A local public IP is required by the real outbound policy'
        self.directory.mkdir(parents=True, mode=0o700)
        self.stack.callback(shutil.rmtree, self.directory, True)

    def run(self, command, *, env=None, cwd=ROOT, input=None, capture=False):
        result = subprocess.run(command, cwd=cwd, env=env or self.environment,
            input=input, text=input is not None or capture, capture_output=True)
        if result.returncode:
            raise AssertionError(f'TC011 subprocess failed: {Path(command[0]).name}, exit {result.returncode}')
        return result.stdout if capture else None

    def sql(self, database, query):
        assert database in self.databases, 'SQL must target a fixture-owned database'
        env = dict(self.environment, PGDATABASE=database)
        return self.run([os.getenv('E2E_PSQL_BIN','psql'), '-X','-v','ON_ERROR_STOP=1','-At'],
            env=env, input=query, capture=True).strip()

    def create_database(self, suffix):
        database = self.name + '_' + suffix
        self.run(['psql','-X','-v','ON_ERROR_STOP=1','-c','CREATE DATABASE ' + database])
        self.databases.append(database)
        self.stack.callback(self.drop_database, database)
        return database

    def drop_database(self, database):
        assert database in self.databases and database.startswith(self.name + '_')
        self.run(['psql','-X','-v','ON_ERROR_STOP=1','-c','DROP DATABASE ' + database + ' WITH (FORCE)'])

    @staticmethod
    def port():
        with socket.socket() as listener:
            listener.bind(('127.0.0.1',0))
            return listener.getsockname()[1]

    def build_packages(self):
        packages = self.directory / 'packages'
        packages.mkdir()
        provider = self.directory / 'provider'
        shutil.copytree(Path(__file__).parents[1] / 'local-entitlement-provider', provider)
        with (provider/'go.mod').open('a') as out:
            out.write('\nreplace lina-core => ' + str(ROOT/'apps/lina-core') + '\n')
        build_env = dict(self.environment, GOWORK='off')
        self.run(['go','mod','tidy'],cwd=provider,env=build_env)
        for source in [ROOT/'apps/lina-plugins/idc-peer',provider]:
            self.run(['go','run','.','wasm','--dir='+str(source),'--out='+str(packages)],
                cwd=ROOT/'hack/tools/linactl',env=build_env)
        return packages

    def start(self, suffix, packages, ca):
        database = self.create_database(suffix)
        directory = self.directory / suffix
        directory.mkdir()
        config = yaml.safe_load((ROOT/'apps/lina-core/manifest/config/config.yaml').read_text())
        # Use the explicit test PG connection, never a deployment database embedded in config.
        env = self.environment
        config['database']['default']['link'] = (f"pgsql:{quote(env['PGUSER'],safe='')}:{quote(env['PGPASSWORD'],safe='')}@"
            f"tcp({env['PGHOST']}:{env['PGPORT']})/{database}?sslmode={env['PGSSLMODE']}")
        port = self.port()
        config['server']['address'] = ':' + str(port)
        config['upload']['path'] = str(directory/'upload')
        config['plugin']['dynamic']['storagePath'] = str(directory/'dynamic')
        config['plugin']['autoEnable'] = [{'id':name,'withMockData':False} for name in ['tenant-core','org-core','idc-core','idc-resource','idc-channel']]
        shutil.copytree(ROOT/'temp/output',directory/'dynamic')
        shutil.copytree(packages,directory/'dynamic',dirs_exist_ok=True)
        config_file = directory/'config.yaml'
        config_file.write_text(yaml.safe_dump(config,allow_unicode=True));config_file.chmod(0o600)
        runtime = dict(env, GF_GCFG_PATH=str(directory),GF_GCFG_FILE='config.yaml',
            SSL_CERT_FILE=str(ca),LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN=f'http://127.0.0.1:{port}')
        runtime['LINAPRO_IDC_CORE_SECURITY_KEYRING'] = (ROOT/'temp/runtime-security.json').read_text()
        for key, file in [('LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE','gateway-auth.json'),
            ('LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE','installation-id')]:
            runtime[key] = str(ROOT/'temp'/file)
        runtime['LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW']='60s'
        self.run([str(ROOT/'temp/bin/lina'),'init','--confirm=init','--sql-source=embedded'],cwd=ROOT/'apps/lina-core',env=runtime)
        log = self.stack.enter_context((directory/'runtime.log').open('wb'))
        process = subprocess.Popen([str(ROOT/'temp/bin/lina')],cwd=ROOT/'apps/lina-core',
            env=runtime,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
        self.stack.callback(self.stop,process)
        deadline=time.monotonic()+60
        while time.monotonic()<deadline:
            if process.poll() is not None:
                diagnostic=self.directory.parent/(self.directory.name+'-'+suffix+'-startup.log')
                shutil.copyfile(directory/'runtime.log',diagnostic);diagnostic.chmod(0o600)
                raise AssertionError('Isolated server exited during startup; private diagnostic saved')
            try:
                with socket.create_connection(('127.0.0.1',port),timeout=1):
                    return API(port),database
            except OSError:
                time.sleep(.2)
        raise AssertionError('Isolated server did not become ready')

    @staticmethod
    def stop(process):
        if process.poll() is None:
            os.killpg(process.pid,signal.SIGTERM)
            try: process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid,signal.SIGKILL);process.wait(timeout=5)

    def certificate(self):
        ca=self.directory/'certificate.pem';key=self.directory/'private-key.pem'
        self.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1',
            '-keyout',str(key),'-out',str(ca),'-subj','/CN=TC011 local peer',
            '-addext','subjectAltName=IP:'+self.public_ip])
        key.chmod(0o600)
        return ca,key

    def proxy(self, upstream, ca, key):
        upstream_port=urlparse(upstream.origin).port
        class Handler(http.server.BaseHTTPRequestHandler):
            def forward(self):
                data=self.rfile.read(int(self.headers.get('Content-Length','0')))
                headers={k:v for k,v in self.headers.items() if k.lower() not in ['host','connection','transfer-encoding']}
                conn=http.client.HTTPConnection('127.0.0.1',upstream_port,timeout=120)
                try:
                    conn.request(self.command,self.path,data,headers)
                    response=conn.getresponse();body=response.read();self.send_response(response.status)
                    for k,v in response.getheaders():
                        if k.lower() not in ['content-length','transfer-encoding','connection']:self.send_header(k,v)
                    self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
                finally:conn.close()
            do_GET=forward;do_POST=forward;do_PUT=forward;do_DELETE=forward
            def log_message(self,*args):pass
        address=self.public_ip
        class Server(http.server.ThreadingHTTPServer):
            address_family=socket.AF_INET6 if ':' in address else socket.AF_INET
        server=Server((address,0),Handler)
        context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(ca,key)
        server.socket=context.wrap_socket(server.socket,server_side=True)
        thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        self.stack.callback(server.server_close);self.stack.callback(server.shutdown)
        address='['+address+']' if ':' in address else address
        return f'https://{address}:{server.server_port}'

    def __enter__(self): return self
    def __exit__(self,*args):
        if args and args[0] is not None:
            for suffix in ['up','down']:
                source=self.directory/suffix/'runtime.log'
                if source.exists():
                    diagnostic=self.directory.parent/(self.directory.name+'-'+suffix+'-failure.log')
                    shutil.copyfile(source,diagnostic);diagnostic.chmod(0o600)
        return self.stack.__exit__(*args)
