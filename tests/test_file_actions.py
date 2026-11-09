import base64,json,os,re,socket,struct,subprocess,sys,tempfile,threading,unittest
from pathlib import Path
DELETE=re.search(r'const deleteFilePython = `([\s\S]*?)`',Path('file_actions.go').read_text()).group(1)
RCON=re.search(r'const rconPython = `([\s\S]*?)`',Path('server.go').read_text()).group(1)
def revision(path):
 s=os.lstat(path);return f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
class FileAndConsoleTests(unittest.TestCase):
 def test_delete_only_selected_unchanged_file(self):
  with tempfile.TemporaryDirectory() as d:
   a=Path(d)/"plugin's JAR.jar";b=Path(d)/'keep.txt';a.write_text('jar');b.write_text('keep')
   result=subprocess.run([sys.executable,'-c',DELETE,str(a),revision(a),d],capture_output=True)
   self.assertEqual(result.returncode,0,result.stderr);self.assertFalse(a.exists());self.assertTrue(b.exists())
 def test_stale_file_and_directory_are_rejected(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'data';p.write_text('before');old=revision(p);p.write_text('after')
   self.assertNotEqual(subprocess.run([sys.executable,'-c',DELETE,str(p),old,d],capture_output=True).returncode,0)
   self.assertEqual(p.read_text(),'after')
   self.assertNotEqual(subprocess.run([sys.executable,'-c',DELETE,d,revision(d),d],capture_output=True).returncode,0)
 def test_symlink_delete_keeps_target(self):
  with tempfile.TemporaryDirectory() as d:
   target=Path(d)/'keep';target.write_text('keep');link=Path(d)/'link';link.symlink_to(target)
   r=subprocess.run([sys.executable,'-c',DELETE,str(link),revision(link),d],capture_output=True)
   self.assertEqual(r.returncode,0,r.stderr);self.assertEqual(target.read_text(),'keep')

 def test_delete_nonempty_folder_and_keep_other_folders(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)
   target=root/'old-plugins';(target/'nested').mkdir(parents=True)
   (target/'nested'/'file.jar').write_bytes(b'jar')
   keep=root/'world';keep.mkdir();(keep/'level.dat').write_bytes(b'world')
   result=subprocess.run([sys.executable,'-c',DELETE,str(target),revision(target),d],capture_output=True)
   self.assertEqual(result.returncode,0,result.stderr)
   self.assertFalse(target.exists())
   self.assertEqual((keep/'level.dat').read_bytes(),b'world')
 def test_reject_folder_outside_selected_instance_and_symlink_target(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);instance=root/'instance';instance.mkdir()
   sibling=root/'other';sibling.mkdir()
   (sibling/'important.txt').write_text('keep')
   outside=subprocess.run([sys.executable,'-c',DELETE,str(sibling),revision(sibling),str(instance)],capture_output=True)
   self.assertNotEqual(outside.returncode,0);self.assertTrue(sibling.exists())
   link=instance/'link';link.symlink_to(sibling,target_is_directory=True)
   # A symlink is deleted as a link; target directory is not followed.
   r=subprocess.run([sys.executable,'-c',DELETE,str(link),revision(link),str(instance)],capture_output=True)
   self.assertEqual(r.returncode,0,r.stderr);self.assertTrue(sibling.exists())
 def test_rcon_preserves_command_and_collects_multiple_packets(self):
  with tempfile.TemporaryDirectory() as d:
   listener=socket.socket();listener.bind(('127.0.0.1',0));listener.listen();self.addCleanup(listener.close)
   Path(d,'server.properties').write_text(f'enable-rcon=true\nrcon.password=test-only\nrcon.port={listener.getsockname()[1]}\n')
   commands=[]
   def receive(s,n):
    out=b''
    while len(out)<n:out+=s.recv(n-len(out))
    return out
   def packet(s):
    n=struct.unpack('<i',receive(s,4))[0];data=receive(s,n);return struct.unpack('<ii',data[:8]),data[8:-2]
   def send(s,id,kind,text):
    data=struct.pack('<ii',id,kind)+text+b'\0\0';s.sendall(struct.pack('<i',len(data))+data)
   def serve():
    s,_=listener.accept()
    with s:
     (id,kind),pw=packet(s);send(s,id,0,b'');send(s,id,2,b'')
     (id,kind),cmd=packet(s);commands.append(cmd)
     send(s,id,0,b'first part ');send(s,id,0,b'second part')
   thread=threading.Thread(target=serve);thread.start()
   r=subprocess.run([sys.executable,'-c',RCON,d,base64.b64encode(b'corestatus').decode()],capture_output=True,timeout=10);thread.join(timeout=2)
   self.assertEqual(r.returncode,0,r.stderr);self.assertEqual(r.stdout.strip(),b'first part second part');self.assertEqual(commands,[b'corestatus'])
if __name__=='__main__':unittest.main()
