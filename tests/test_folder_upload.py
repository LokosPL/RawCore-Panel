import json,os,re,subprocess,sys,tempfile,unittest
from pathlib import Path
SCRIPT=re.search(r'const mkdirTreePython = \`([\s\S]*?)\`',Path('folder_upload.go').read_text()).group(1)
class FolderUploadTests(unittest.TestCase):
 def run_script(self,path,root):
  return subprocess.run([sys.executable,'-c',SCRIPT,str(path),str(root)],capture_output=True)
 def test_creates_nested_directories_and_is_idempotent(self):
  with tempfile.TemporaryDirectory() as d:
   dest=Path(d)/'bundle'/'sub'/'plugins'
   self.assertEqual(self.run_script(dest,d).returncode,0)
   self.assertTrue(dest.is_dir())
   self.assertEqual(self.run_script(dest,d).returncode,0)
 def test_rejects_outside_path(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)/'instance';root.mkdir()
   sibling=Path(d)/'other';sibling.mkdir()
   self.assertNotEqual(self.run_script(sibling/'danger',root).returncode,0)
   self.assertFalse((sibling/'danger').exists())
 def test_rejects_symlinks_and_files(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)/'instance';root.mkdir()
   elsewhere=Path(d)/'elsewhere';elsewhere.mkdir()
   (root/'escape').symlink_to(elsewhere,target_is_directory=True)
   self.assertNotEqual(self.run_script(root/'escape'/'sub',root).returncode,0)
   self.assertFalse((elsewhere/'sub').exists())
   (root/'file').write_text('x')
   self.assertNotEqual(self.run_script(root/'file'/'sub',root).returncode,0)
   self.assertEqual((root/'file').read_text(),'x')
if __name__=='__main__':unittest.main()
