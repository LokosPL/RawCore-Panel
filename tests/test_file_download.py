import io,json,os,re,subprocess,sys,tempfile,unittest,zipfile
from pathlib import Path
SOURCE=Path('file_download.go').read_text()
INFO=re.search(r'const downloadInfoPython = \`([\s\S]*?)\`',SOURCE).group(1)
DOWNLOAD=re.search(r'const downloadStreamPython = \`([\s\S]*?)\`',SOURCE).group(1)
class DownloadTests(unittest.TestCase):
 def test_download_bytes_including_binary(self):
  with tempfile.TemporaryDirectory() as d:
   path=Path(d)/'Core.jar';raw=bytes(range(256))*1024;path.write_bytes(raw)
   meta=json.loads(subprocess.check_output([sys.executable,'-c',INFO,str(path)]))
   self.assertFalse(meta['directory']);self.assertEqual(meta['size'],len(raw))
   received=subprocess.check_output([sys.executable,'-c',DOWNLOAD,str(path),'file'])
   self.assertEqual(received,raw)
 def test_directory_zip_and_symlink_safety(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);folder=root/'plugins';(folder/'sub').mkdir(parents=True)
   (folder/'sub'/'ViaVersion.jar').write_bytes(b'\0\xffhello')
   (folder/'empty').mkdir()
   (root/'private').write_bytes(b'secret')
   (folder/'link').symlink_to(root/'private')
   (folder/'outside').symlink_to(root,target_is_directory=True)
   meta=json.loads(subprocess.check_output([sys.executable,'-c',INFO,str(folder)]))
   self.assertTrue(meta['directory'])
   result=subprocess.check_output([sys.executable,'-c',DOWNLOAD,str(folder),'directory'])
   with zipfile.ZipFile(io.BytesIO(result)) as z:
    self.assertIn('plugins/sub/ViaVersion.jar',z.namelist())
    self.assertIn('plugins/empty/',z.namelist())
    self.assertNotIn('plugins/link',z.namelist())
    self.assertNotIn('plugins/outside/',z.namelist())
    self.assertEqual(z.read('plugins/sub/ViaVersion.jar'),b'\0\xffhello')
    self.assertTrue(all(not n.startswith('/') and '..' not in n.split('/') for n in z.namelist()))
 def test_missing_file_and_special_path_fail(self):
  with tempfile.TemporaryDirectory() as d:
   missing=Path(d)/'missing'
   self.assertNotEqual(subprocess.run([sys.executable,'-c',INFO,str(missing)],capture_output=True).returncode,0)
   self.assertNotEqual(subprocess.run([sys.executable,'-c',DOWNLOAD,str(missing),'file'],capture_output=True).returncode,0)
if __name__=='__main__':unittest.main()
