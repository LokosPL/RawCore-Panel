import http.server,json,os,re,subprocess,tempfile,threading,time,unittest
from pathlib import Path
SCRIPT=re.search(r'const paymentsPython = `([\s\S]*?)`',Path('payments.go').read_text()).group(1)
class PaymentStatusTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name);self.folder=self.root/'plugins/Core/website';self.folder.mkdir(parents=True)
  self.requests=[];self.payload={'version':1,'bridgeEnabled':True,'paymentsEnabled':True,'testMode':False,'operational':True,'bridgeConnected':True,'acceptingTopups':True,'paymentsAvailable':True,'lastBridgeSuccessAt':int(time.time()*1000),'observedAt':int(time.time()*1000),'reason':'READY','api-key':'DO-NOT-EXPOSE','accountId':'private-account'}
  owner=self
  class Handler(http.server.BaseHTTPRequestHandler):
   def do_POST(self):
    owner.requests.append((self.path,self.headers.get('Authorization'),self.rfile.read(int(self.headers['Content-Length']))));raw=json.dumps(owner.payload).encode();self.send_response(200);self.end_headers();self.wfile.write(raw)
   def log_message(self,*args):pass
  self.server=http.server.HTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=self.server.serve_forever,daemon=True).start();self.addCleanup(self.server.server_close);self.addCleanup(self.server.shutdown)
  self.config=self.folder/'server.properties';self.write()
 def write(self,bind='127.0.0.1',mode=0o600):
  self.config.write_text(f'bind={bind}\nport={self.server.server_port}\napi-key={"a"*40}\n');self.config.chmod(mode)
 def read(self):
  p=subprocess.run(['python3','-c',SCRIPT,str(self.root)],capture_output=True,text=True,timeout=8);self.assertNotIn('DO-NOT-EXPOSE',p.stdout+p.stderr);return json.loads(p.stdout)
 def test_authenticated_loopback_status_strips_secrets_and_account_data(self):
  data=self.read();self.assertTrue(data['paymentsAvailable']);self.assertNotIn('api-key',data);self.assertNotIn('accountId',data);self.assertEqual(self.requests,[('/v1/admin/payments/status','Bearer '+'a'*40,b'{}')])
 def test_redirect_or_external_bind_cannot_receive_the_key(self):
  self.write(bind='example.com');self.assertIn('error',self.read());self.assertEqual(self.requests,[])
 def test_bad_permissions_and_symlink_configuration_fail_closed(self):
  self.write(mode=0o644);self.assertIn('error',self.read());self.write();self.config.rename(self.folder/'real.properties');self.config.symlink_to(self.folder/'real.properties');self.assertIn('error',self.read());self.assertEqual(self.requests,[])
 def test_stale_or_untyped_response_is_never_available(self):
  self.payload['observedAt']=1;self.assertIn('error',self.read());self.payload['observedAt']=int(time.time()*1000);self.payload['paymentsAvailable']='true';self.assertIn('error',self.read())
 def test_inconsistent_green_status_is_rejected(self):
  self.payload['operational']=False;self.assertIn('error',self.read())
  self.payload['operational']=True;self.payload['testMode']=True;self.assertIn('error',self.read())
  self.payload['testMode']=False;self.payload['reason']='MAINTENANCE';self.assertIn('error',self.read())
if __name__=='__main__':unittest.main()
