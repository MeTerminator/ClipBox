"""Verify portable archives contain the intended executable and no runtime data."""

import hashlib
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

from package_desktop import TARGETS, package_desktop


class PortablePackageTests(unittest.TestCase):
    def test_platform_archives_and_checksums(self):
        for (platform, arch), target in TARGETS.items():
            with self.subTest(platform=platform, arch=arch), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                binary_name = "ClipBox.exe" if platform == "windows" else "ClipBox"
                release = root / "frontend/src-tauri/target" / target / "release"
                release.mkdir(parents=True)
                (release / binary_name).write_bytes(b"portable executable")
                (release / "private-token.txt").write_text("must not be archived")
                (root / "docs").mkdir()
                (root / "data").mkdir()
                (root / "data/secret.txt").write_text("must not be archived")
                for document in ("LICENSE", "README.md", "README_CN.md", "docs/PORTABLE.md"):
                    (root / document).write_text("instructions", encoding="utf-8")
                archive = package_desktop(root, "261008", platform, arch, target)
                name = f"clipbox-client-261008-{platform}-{arch}-portable"
                expected = {f"{name}/{document}" for document in
                            (binary_name, "LICENSE", "README.md", "README_CN.md", "docs/PORTABLE.md")}
                if platform == "windows":
                    with zipfile.ZipFile(archive) as output:
                        self.assertEqual(set(output.namelist()), expected)
                        self.assertEqual(output.read(f"{name}/{binary_name}"), b"portable executable")
                else:
                    with tarfile.open(archive) as output:
                        self.assertEqual({item.name for item in output if item.isfile()}, expected)
                        self.assertEqual(output.getmember(f"{name}/{binary_name}").mode, 0o755)
                        self.assertEqual(output.extractfile(f"{name}/{binary_name}").read(), b"portable executable")
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                self.assertEqual(archive.with_name(f"{archive.name}.sha256").read_text(),
                                 f"{digest}  {archive.name}\n")

    def test_rejects_unsafe_versions_and_mismatched_targets(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for version in ("../escape", "", "bad/version", "line\nbreak"):
                with self.assertRaises(ValueError):
                    package_desktop(root, version, "windows", "amd64", TARGETS[("windows", "amd64")])
            with self.assertRaises(ValueError):
                package_desktop(root, "261008", "windows", "amd64", TARGETS[("macos", "amd64")])
            with self.assertRaises(FileNotFoundError):
                package_desktop(root, "261008", "windows", "amd64", TARGETS[("windows", "amd64")])


if __name__ == "__main__":
    unittest.main()
