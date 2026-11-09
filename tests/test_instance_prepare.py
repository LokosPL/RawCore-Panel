import json,os,re,socket,subprocess,sys,tempfile,unittest
from pathlib import Path
SCRIPT=re.search(r'const prepareInstancePython = `([\s\S]*?)`',Path('instance_prepare.go').read_text()).group(1)
class PrepareInstanceTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name).resolve();self.main=self.root/'normalny';self.main.mkdir();(self.main/'server.jar').write_bytes(b'jar-fixture');(self.main/'secret').write_text('production-secret');self.target=self.root/'testowy';self.java=self.root/'java';self.java.write_text('#!'+sys.executable+'\nimport sys\nprint(\'openjdk version "25"\',file=sys.stderr)\n');self.java.chmod(0o755)
  self.bin=self.root/'bin';self.bin.mkdir();pm2=self.bin/'pm2';pm2.write_text('#!'+sys.executable+'\nprint("[]")\n');pm2.chmod(0o755);self.env={**os.environ,'PATH':str(self.bin)+os.pathsep+os.environ.get('PATH','')}
  def free():
   with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
  a,b=free(),free();c=free()
  while a==b:b=free()
  while c in (a,b):c=free()
  self.req={'instance':{'id':'testowy','name':'Test','serverPath':str(self.target),'minecraftProcess':'mc-test','wwwProcess':'','port':a,'rconPort':b,'corePort':c,'java':str(self.java),'ram':2048,'version':'Paper fixture'},'sourceJar':str(self.main/'server.jar'),'existing':[{'id':'normalny','serverPath':str(self.main)}],'action':'preview'}
 def run_script(self):return subprocess.run([sys.executable,'-c',SCRIPT,json.dumps(self.req)],capture_output=True,text=True,env=self.env,timeout=15)
 def test_preview_does_not_write_then_prepare_is_isolated_and_does_not_start_pm2(self):
  r=self.run_script();self.assertEqual(r.returncode,0,r.stderr);self.assertFalse(self.target.exists());self.req['action']='create';r=self.run_script();self.assertEqual(r.returncode,0,r.stderr);self.assertFalse(json.loads(r.stdout)['started']);self.assertFalse((self.target/'secret').exists());self.assertFalse((self.target/'plugins/Core.jar').exists());self.assertEqual(json.loads((self.target/'plugins/Core/DataBase.json').read_text())['type'],'SQLITE');self.assertIn('port='+str(self.req['instance']['corePort']),(self.target/'plugins/Core/website/server.properties').read_text());self.assertIn('wlaczone: false',(self.target/'plugins/Core/website/integration.yml').read_text());self.assertIn('eula=false',(self.target/'eula.txt').read_text());self.assertIn('accepts-transfers=true',(self.target/'server.properties').read_text());self.assertFalse(json.loads((self.target/'plugins/Core/Voting.json').read_text())['enabled']);self.assertEqual((self.main/'secret').read_text(),'production-secret');self.assertNotEqual(self.run_script().returncode,0)
 def test_occupied_port_and_existing_target_are_rejected(self):
  with socket.socket() as sock:
   sock.bind(('0.0.0.0',self.req['instance']['port']));self.assertNotEqual(self.run_script().returncode,0)
  self.target.mkdir();(self.target/'keep').write_text('safe');self.req['action']='create';self.assertNotEqual(self.run_script().returncode,0);self.assertEqual((self.target/'keep').read_text(),'safe')
 def test_stopped_instance_ports_are_reserved_from_properties(self):
  (self.main/'server.properties').write_text('server-port='+str(self.req['instance']['port'])+'\n')
  self.assertNotEqual(self.run_script().returncode,0)
  (self.main/'server.properties').write_text('server-port=25565\n')
  folder=self.main/'plugins/Core/website';folder.mkdir(parents=True);(folder/'server.properties').write_text('port='+str(self.req['instance']['corePort'])+'\n')
  self.assertNotEqual(self.run_script().returncode,0)
 def test_source_symlink_is_rejected(self):
  link=self.main/'alias.jar';link.symlink_to(self.main/'server.jar');self.req['sourceJar']=str(link);self.assertNotEqual(self.run_script().returncode,0)
if __name__=='__main__':unittest.main()
