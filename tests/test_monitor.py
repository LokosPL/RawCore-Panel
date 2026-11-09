import unittest,tempfile,subprocess,json,re,time,os,datetime
from pathlib import Path
SCRIPT=re.search(r'const monitorPython = `([\s\S]*?)`',Path('monitor.go').read_text()).group(1)
class MonitorTests(unittest.TestCase):
 def run_monitor(self,path,lines,started=None,scan=False):
  log=path/'latest.log';log.write_text(lines)
  request={'processes':[{'pm_id':1,'name':'minecraft','pm2_env':{'pm_cwd':str(path),'pm_out_log_path':str(log),'pm_uptime':(started or time.time()-60)*1000}}],'serverPath':str(path),'minecraftProcess':'minecraft'}
  if not scan: request['disks']=[]
  return json.loads(subprocess.check_output(['python3','-c',SCRIPT,json.dumps(request)],text=True))
 def test_errors_and_start_block_deduplicate_repeated_timestamps(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d);date=datetime.datetime.now().strftime('%Y-%m-%d')
   out=self.run_monitor(p,f'{date} 23:00:01 +00:00: [23:00:01 WARN]: [Core] [START BLOCK] Dane=false\n{date} 23:00:02 +00:00: [23:00:02 WARN]: [Core] [START BLOCK] Dane=false\n',started=time.time()-86400)
   # Build recent timestamps rather than depending on the time this suite runs.
   now=datetime.datetime.now().astimezone();a=now-datetime.timedelta(seconds=4);b=now-datetime.timedelta(seconds=2)
   lines=''.join(x.strftime('%Y-%m-%d %H:%M:%S %z: [%H:%M:%S WARN]: [Core] [START BLOCK] Dane=false\n') for x in [a,b])
   out=self.run_monitor(p,lines)
   self.assertEqual(len(out['services'][0]['errors']),1)
 def test_no_false_alarm_for_info_warn_and_zero_error_counters(self):
  with tempfile.TemporaryDirectory() as d:
   out=self.run_monitor(Path(d),'[INFO]: 0 errors, error=false\n[WARN]: Waiting for player\n[INFO]: Error handling enabled\n')
   self.assertEqual(out['services'][0]['errors'],[])
 def test_real_error_exception_and_oom_detected(self):
  with tempfile.TemporaryDirectory() as d:
   out=self.run_monitor(Path(d),'[ERROR]: Database unavailable\nError: listen EADDRINUSE\njava.lang.OutOfMemoryError: Java heap space\n')
   self.assertEqual(len(out['services'][0]['errors']),3)
 def test_timestamped_node_and_lowercase_web_errors(self):
  with tempfile.TemporaryDirectory() as d:
   now=datetime.datetime.now().astimezone().isoformat()
   out=self.run_monitor(Path(d),now+' TypeError: Cannot read properties of undefined\n'+now+' [error] connect() failed\n')
   self.assertEqual(len(out['services'][0]['errors']),2)
 def test_old_error_before_restart_ignored(self):
  with tempfile.TemporaryDirectory() as d:
   old=(datetime.datetime.now().astimezone()-datetime.timedelta(hours=2)).isoformat()
   out=self.run_monitor(Path(d),old+' [ERROR]: historical failure\n[INFO]: Ready\n')
   self.assertEqual(out['services'][0]['errors'],[])
 def test_disk_breakdown_and_cached_payload(self):
  with tempfile.TemporaryDirectory() as d:
   path=Path(d);(path/'world').mkdir();(path/'world'/'data').write_bytes(b'x'*32768)
   out=self.run_monitor(path,'[INFO]: Ready\n',scan=True)
   self.assertGreater(out['disks'][0]['bytes'],0)
   self.assertTrue(any(x['path'].endswith('/world') for x in out['disks'][0]['folders']))
 def test_symlink_service_directory_measures_target_contents(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d);target=p/'actual';target.mkdir();(target/'world').mkdir();(target/'world'/'region').write_bytes(b'x'*32768);link=p/'server';link.symlink_to(target,target_is_directory=True)
   out=self.run_monitor(link,'[INFO]: Ready\n',scan=True)
   self.assertGreater(out['disks'][0]['bytes'],32768)
   self.assertEqual(out['disks'][0]['path'],str(link))
   self.assertTrue(any(f['path']==str(link/'world') for f in out['disks'][0]['folders']))
 def test_shared_directory_reported_once_and_missing_logs_explicit(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d);req={'processes':[{'pm_id':i,'name':str(i),'pm2_env':{'pm_cwd':d,'pm_err_log_path':str(p)}} for i in [1,2]],'serverPath':d,'minecraftProcess':'mc'}
   out=json.loads(subprocess.check_output(['python3','-c',SCRIPT,json.dumps(req)],text=True))
   self.assertEqual(len(out['disks']),1);self.assertTrue(out['services'][0]['issues'])
if __name__=='__main__':unittest.main()
