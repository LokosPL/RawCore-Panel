"""Exercise the actual Python programs sent through SSH against real files."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path('file_editor.go').read_text()
READ = re.search(r'const readChunkPython = `([\s\S]*?)`', SOURCE).group(1)
SAVE = re.search(r'const saveChunkPython = `([\s\S]*?)`', SOURCE).group(1)
CHUNK = 256 * 1024

class RemoteEditorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "a 'quoted' file.dat"

    def read(self, offset=0):
        return json.loads(subprocess.check_output([sys.executable, '-c', READ, str(self.path), str(offset), str(CHUNK)]))

    def save(self, chunk, replacement, script=SAVE):
        return subprocess.run([sys.executable, '-c', script, str(self.path), str(chunk['offset']), str(chunk['length']), chunk['hash'], chunk['version']], input=replacement, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_large_file_read_is_bounded_and_end_is_editable(self):
        with self.path.open('wb') as f:
            f.seek(64 * 1024 * 1024)
            f.write(b'END')
        first = self.read()
        self.assertEqual(first['size'], 64 * 1024 * 1024 + 3)
        self.assertEqual(first['length'], CHUNK)
        last = self.read(64 * 1024 * 1024)
        result = self.save(last, b'UPDATED-END')
        self.assertEqual(result.returncode, 0, result.stderr)
        with self.path.open('rb') as f:
            f.seek(64 * 1024 * 1024)
            self.assertEqual(f.read(), b'UPDATED-END')
        self.assertEqual(Path(str(self.path)+'.rawcraft.bak').stat().st_size, first['size'])

    def test_binary_bytes_and_unedited_tail_are_preserved(self):
        original = bytes(range(256)) * 3000
        self.path.write_bytes(original)
        os.chmod(self.path, 0o640)
        part = self.read()
        self.assertTrue(part['binary'])
        self.assertEqual(base64.b64decode(part['data']), original[:CHUNK])
        replacement = b'\x00\xff\x80new'
        result = self.save(part, replacement)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.path.read_bytes(), replacement + original[CHUNK:])
        self.assertEqual(Path(str(self.path)+'.rawcraft.bak').read_bytes(), original)
        self.assertEqual(stat.S_IMODE(self.path.stat().st_mode), 0o640)

    def test_utf8_boundary_and_crlf_bytes(self):
        original = b'x' * (CHUNK - 1) + 'ą'.encode() + b'\r\nEND\r\n'
        self.path.write_bytes(original)
        part = self.read()
        self.assertFalse(part['binary'])
        self.assertEqual(part['next'], CHUNK - 1)
        tail = self.read(part['next'])
        self.assertEqual(base64.b64decode(part['data']) + base64.b64decode(tail['data']), original)
        result = self.save(tail, 'ą\r\nCHANGED\r\n'.encode())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.path.read_bytes().endswith('ą\r\nCHANGED\r\n'.encode()))

    def test_concurrent_change_is_rejected_without_overwrite(self):
        self.path.write_bytes(b'original')
        part = self.read()
        self.path.write_bytes(b'external change')
        result = self.save(part, b'my edit')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.path.read_bytes(), b'external change')
        self.assertFalse(Path(str(self.path)+'.rawcraft.bak').exists())

    def test_failed_atomic_replace_keeps_original(self):
        self.path.write_bytes(b'original')
        part = self.read()
        injected = SAVE.replace("os.replace(temp,p)", "(_ for _ in ()).throw(OSError('simulated write failure'))")
        result = self.save(part, b'changed', injected)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.path.read_bytes(), b'original')
        self.assertFalse(list(self.path.parent.glob('.rawcraft-*')))

    def test_empty_file_can_receive_content(self):
        self.path.write_bytes(b'')
        part = self.read()
        self.assertEqual(part['length'], 0)
        result = self.save(part, b'new file\n')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.path.read_bytes(), b'new file\n')

if __name__ == '__main__':
    unittest.main()
