"""Package a Tauri executable without installers or local runtime data."""

import argparse
import hashlib
from pathlib import Path
import re
import shutil
import tarfile
import zipfile

TARGETS = {
    ("windows", "amd64"): "x86_64-pc-windows-msvc",
    ("macos", "amd64"): "x86_64-apple-darwin",
    ("macos", "arm64"): "aarch64-apple-darwin",
    ("linux", "amd64"): "x86_64-unknown-linux-gnu",
    ("linux", "arm64"): "aarch64-unknown-linux-gnu",
}


def package_desktop(root: Path, version: str, platform: str, arch: str, target: str) -> Path:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", version):
        raise ValueError("Invalid archive version")
    if TARGETS.get((platform, arch)) != target:
        raise ValueError("Platform, architecture and Rust target must match")
    binary_name = "ClipBox.exe" if platform == "windows" else "ClipBox"
    binary = root / "frontend/src-tauri/target" / target / "release" / binary_name
    if not binary.is_file():
        raise FileNotFoundError(f"Built executable missing: {binary}")

    name = f"clipbox-client-{version}-{platform}-{arch}-portable"
    dist = root / "dist"
    stage = dist / name
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir(parents=True)
    shutil.copy2(binary, stage / binary_name)
    if platform != "windows":
        (stage / binary_name).chmod(0o755)
    for document in ("LICENSE", "README.md", "README_CN.md"):
        shutil.copy2(root / document, stage / document)
    (stage / "docs").mkdir()
    shutil.copy2(root / "docs/PORTABLE.md", stage / "docs/PORTABLE.md")

    if platform == "windows":
        archive = dist / f"{name}.zip"
        with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as output:
            for file in sorted(stage.rglob("*")):
                if file.is_file():
                    output.write(file, file.relative_to(dist).as_posix())
    else:
        archive = dist / f"{name}.tar.gz"
        def executable_permissions(info: tarfile.TarInfo) -> tarfile.TarInfo:
            # Windows chmod cannot preserve Unix executable bits on disk.
            if info.name == f"{name}/{binary_name}":
                info.mode = 0o755
            return info

        with tarfile.open(archive, "w:gz") as output:
            output.add(stage, arcname=name, filter=executable_permissions)
    with archive.open("rb") as file:
        digest = hashlib.file_digest(file, "sha256").hexdigest()
    archive.with_name(f"{archive.name}.sha256").write_text(
        f"{digest}  {archive.name}\n", encoding="utf-8", newline="\n"
    )
    return archive


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for argument in ("version", "platform", "arch", "target"):
        parser.add_argument(f"--{argument}", required=True)
    args = parser.parse_args()
    repository = Path(__file__).resolve().parents[2]
    print(package_desktop(repository, args.version, args.platform, args.arch, args.target))
