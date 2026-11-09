import json
import os
from pathlib import Path
import pwd
import re
import shlex
import subprocess
import sys
import tempfile
import unittest

def program(file,name):
    return re.search(r'const '+name+r' = `([\s\S]*?)`',Path(file).read_text()).group(1)
SUDO=program('security.go','sudoPython')
PASSWORD=program('account.go','passwordPython')

class SecurityPrograms(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);self.env=os.environ.copy();self.env['PATH']=str(self.root)+os.pathsep+self.env.get('PATH','')
    def executable(self,name,text):
        p=self.root/name;p.write_text('#!'+sys.executable+'\n'+text);p.chmod(0o755);return p
    def sudo(self,mode,password='correct',payload=b''):
        self.executable('sudo',r'''import sys,os,subprocess
args=sys.argv[1:];prompt=args[args.index('-p')+1]
if os.environ['FAKE_SUDO_MODE']=='lecture': sys.stderr.write('Administrative policy lecture\n');sys.stderr.flush()
if os.environ['FAKE_SUDO_MODE']=='password':
 sys.stderr.write(prompt);sys.stderr.flush()
 if sys.stdin.buffer.readline()!=b'correct\n': sys.stderr.write('Sorry, try again.\n');raise SystemExit(1)
command=args[args.index('--')+1:]
raise SystemExit(subprocess.run(command,stdin=sys.stdin.buffer).returncode)
''')
        import base64
        command=shlex.quote(sys.executable)+" -c "+shlex.quote('import sys;sys.stdout.buffer.write(sys.stdin.buffer.read())')
        env=self.env.copy();env['FAKE_SUDO_MODE']=mode
        return subprocess.run([sys.executable,'-c',SUDO,base64.b64encode(command.encode()).decode()],input=json.dumps({'password':password}).encode()+b'\n'+payload,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env,timeout=20)
    def test_password_sudo_preserves_binary_payload(self):
        data=bytes(range(256))*4096;r=self.sudo('password',payload=data)
        self.assertEqual(r.returncode,0,r.stderr);self.assertEqual(r.stdout,data);self.assertNotIn(b'correct',r.stderr)
    def test_nopasswd_or_cached_sudo_does_not_inject_password_into_file(self):
        data=b'FILE\x00\xff\n';r=self.sudo('nopasswd',payload=data)
        self.assertEqual(r.returncode,0,r.stderr);self.assertEqual(r.stdout,data)
    def test_sudo_lecture_is_not_mixed_into_json_stdout(self):
        r=self.sudo('lecture',payload=b'{"items": []}')
        self.assertEqual(r.returncode,0,r.stderr);self.assertEqual(json.loads(r.stdout),{'items': []});self.assertIn(b'Administrative policy lecture',r.stderr)
    def test_wrong_sudo_password_never_runs_the_command(self):
        r=self.sudo('password',password='wrong-secret',payload=b'private file')
        self.assertEqual(r.returncode,77);self.assertEqual(r.stdout,b'');self.assertIn(b'RAWCRAFT_SUDO_DENIED',r.stderr);self.assertNotIn(b'wrong-secret',r.stderr)
    def test_sudo_is_authenticated_for_each_command(self):
        for text in [b'first',b'second']:
            r=self.sudo('password',payload=text);self.assertEqual(r.stdout,text);self.assertEqual(r.returncode,0,r.stderr)
    def test_root_password_update_uses_stdin_not_arguments(self):
        target=self.root/'received'
        self.executable('chpasswd',f'import sys\nfrom pathlib import Path\nassert len(sys.argv)==1\nPath({str(target)!r}).write_bytes(sys.stdin.buffer.read())\n')
        script=PASSWORD.replace('if os.geteuid()==0:','if True:')
        data={'user':'minecraft','password':'new-password-123','old':'old-password'}
        r=subprocess.run([sys.executable,'-c',script],input=json.dumps(data).encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=self.env,timeout=10)
        self.assertEqual(r.returncode,0,r.stderr);self.assertTrue(json.loads(r.stdout)['ok']);self.assertEqual(target.read_bytes(),b'minecraft:new-password-123\n')
    def test_password_rejection_redacts_secrets(self):
        self.executable('chpasswd',"import sys\nsys.stdin.buffer.read()\nsys.stderr.write('Rejected new-password-123 and old-password')\nraise SystemExit(1)\n")
        r=subprocess.run([sys.executable,'-c',PASSWORD.replace('if os.geteuid()==0:','if True:')],input=json.dumps({'user':'minecraft','password':'new-password-123','old':'old-password'}).encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=self.env,timeout=10)
        self.assertNotEqual(r.returncode,0);self.assertNotIn(b'new-password-123',r.stderr);self.assertNotIn(b'old-password',r.stderr)
    def test_self_service_passwd_dialog(self):
        self.executable('passwd',r'''import sys,termios
attrs=termios.tcgetattr(0);attrs[3]&=~termios.ECHO;termios.tcsetattr(0,termios.TCSANOW,attrs)
def ask(prompt):
 sys.stdout.write(prompt);sys.stdout.flush();return sys.stdin.readline().strip()
assert ask('Current password: ')=='old-password'
assert ask('New password: ')=='new-password-123'
assert ask('Retype new password: ')=='new-password-123'
print('passwd: password updated successfully')
''')
        data={'user':pwd.getpwuid(os.geteuid()).pw_name,'password':'new-password-123','old':'old-password'}
        r=subprocess.run([sys.executable,'-c',PASSWORD.replace('if os.geteuid()==0:','if False:')],input=json.dumps(data).encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=self.env,timeout=35)
        self.assertEqual(r.returncode,0,r.stderr);self.assertTrue(json.loads(r.stdout)['ok'])

if __name__=='__main__':unittest.main()
