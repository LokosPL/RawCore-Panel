import ast
import re
import urllib.parse
import unittest
from pathlib import Path

script = re.search(r"const prepareCatalogPython = `([\s\S]*?)`", Path("instance_catalog.go").read_text()).group(1)
tree = ast.parse(script)
# Compile only the same validator used by both initial URLs and HTTP redirects.
host_assignment = next(node for node in ast.walk(tree) if isinstance(node, ast.Assign)
    and any(isinstance(t, ast.Name) and t.id == "allowed_hosts" for t in node.targets))
validator = next(node for node in ast.walk(tree) if isinstance(node, ast.FunctionDef)
    and node.name == "validate_download_url")
namespace = {"urllib": urllib}
exec(compile(ast.fix_missing_locations(ast.Module(body=[host_assignment, validator], type_ignores=[])),
    "<catalog-validator>", "exec"), namespace)

class DownloadHostValidationTests(unittest.TestCase):
    def test_paper_fill_data_url_from_official_v3_api(self):
        url = "https://fill-data.papermc.io/v1/objects/" + ("a" * 64) + "/paper-26.3-189.jar"
        self.assertEqual(namespace["validate_download_url"](url).hostname, "fill-data.papermc.io")

    def test_modrinth_cdn_jar(self):
        url = "https://cdn.modrinth.com/data/abc/versions/def/ViaVersion.jar"
        self.assertEqual(namespace["validate_download_url"](url).hostname, "cdn.modrinth.com")

    def test_reject_other_hosts_and_unsafe_urls(self):
        invalid = [
            "http://fill-data.papermc.io/file.jar",
            "https://fill-data.papermc.io.attacker.invalid/file.jar",
            "https://evil.invalid/server.jar",
            "https://fill-data.papermc.io:8443/file.jar",
            "file:///etc/passwd",
            "https://name:password@fill-data.papermc.io/file.jar",
        ]
        for url in invalid:
            with self.subTest(url=url), self.assertRaises(RuntimeError):
                namespace["validate_download_url"](url)

if __name__ == "__main__":
    unittest.main()
