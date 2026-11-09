import re,subprocess,tempfile,os,unittest
from pathlib import Path
script=re.search(r'const Upload = `([\s\S]*?)`',Path('shared/remotefiles/upload.go').read_text()).group(1)
class AtomicUpload(unittest.TestCase):
 def test_interruption_conflict_backup_and_permissions(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'plugin.jar';p.write_bytes(b'old');p.chmod(0o640)
   def upload(data,size,version=''):
    return subprocess.run(['python3','-c',script,str(p),version,str(size)],input=data,capture_output=True)
   self.assertNotEqual(upload(b'partial',100).returncode,0);self.assertEqual(p.read_bytes(),b'old');self.assertEqual(len(list(Path(d).iterdir())),1)
   self.assertEqual(upload(b'new',3).returncode,0);self.assertEqual(p.read_bytes(),b'new');self.assertEqual((Path(d)/'plugin.jar.rawcraft.bak').read_bytes(),b'old');self.assertEqual(p.stat().st_mode&0o777,0o640)
   self.assertNotEqual(upload(b'bad',3,'old-revision').returncode,0);self.assertEqual(p.read_bytes(),b'new')

 def test_folder_upload_creates_missing_nested_parent_immediately(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'world'/'players'/'stats'/'player.json'
   r=subprocess.run(['python3','-c',script,str(p),'','7',str(d)],input=b'{"x":1}',capture_output=True)
   self.assertEqual(r.returncode,0,r.stderr)
   self.assertEqual(p.read_bytes(),b'{"x":1}')
   # A server may remove old cache/stat directories between file uploads.
   (Path(d)/'world'/'players'/'stats'/'player.json').unlink()
   (Path(d)/'world'/'players'/'stats').rmdir()
   r=subprocess.run(['python3','-c',script,str(p),'','5',str(d)],input=b'fresh',capture_output=True)
   self.assertEqual(r.returncode,0,r.stderr)
   self.assertEqual(p.read_bytes(),b'fresh')
 def test_folder_upload_rejects_outside_root_and_symlinks(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)/'survival';root.mkdir()
   other=Path(d)/'outside';other.mkdir()
   outside=other/'file.txt'
   r=subprocess.run(['python3','-c',script,str(outside),'','4',str(root)],input=b'data',capture_output=True)
   self.assertNotEqual(r.returncode,0);self.assertFalse(outside.exists())
   (root/'link').symlink_to(other,target_is_directory=True)
   r=subprocess.run(['python3','-c',script,str(root/'link'/'file.txt'),'','4',str(root)],input=b'data',capture_output=True)
   self.assertNotEqual(r.returncode,0);self.assertFalse(outside.exists())
if __name__=='__main__':unittest.main()
