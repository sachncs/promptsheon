#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
SRC = re.compile(r"(?:internal|pkg|cmd)/[A-Za-z0-9_./-]+\.go(?::[0-9]+(?:-[0-9]+)?)?|api/openapi\.yaml|deploy/[A-Za-z0-9_./-]+\.(?:yaml|yml|tpl|json|md|sh)|scripts/[A-Za-z0-9_./-]+\.(?:sh|go|py)|\.github/workflows/[A-Za-z0-9_./-]+\.(?:yml|yaml)|docs/adr/[0-9]+-[A-Za-z0-9_.-]+\.md")
LINK = re.compile(r"\[[^\]]*\]\(([^)]+)\)")
SPAN = re.compile(r"`[^`]*`")


def files(exclude_refs=False):
    result = sorted(ROOT.joinpath("docs").rglob("*.md"))
    result += [p for p in (ROOT / "README.md", ROOT / "CHANGELOG.md") if p.is_file()]
    if exclude_refs:
        result = [p for p in result if p.name != "CHANGELOG.md" and "docs/adr" not in p.relative_to(ROOT).as_posix() and p.name != "triz-report.md"]
    return result


def relative_path(file, target):
    if target.startswith("/"):
        return Path(target)
    candidate = file.parent / target
    return candidate if candidate.exists() else ROOT / target


def clean_target(target):
    return target.split("#", 1)[0].split("?", 1)[0]


def main():
    link_files = files()
    ref_files = files(True)
    if not link_files and not ref_files:
        print("docs-check: no markdown files found")
        return 0
    findings = []
    for file in link_files:
        rel = file.relative_to(ROOT).as_posix()
        for target in LINK.findall(file.read_text()):
            if not target or target.startswith("#") or re.match(r"(?:https?|mailto|data|javascript):", target):
                continue
            target = clean_target(target)
            if target and not relative_path(file, target).exists():
                findings.append(f"BROKEN-LINK  {rel}  {target}")
    for file in ref_files:
        rel = file.relative_to(ROOT).as_posix()
        for line in file.read_text().splitlines():
            if re.search(r"<!--[ \t]*stale-ok:", line):
                continue
            candidates = [clean_target(x) for x in LINK.findall(line)]
            candidates += [x[1:-1] for x in SPAN.findall(line)]
            for target in candidates:
                if not target or " " in target or ":" in target or "{" in target:
                    continue
                match = SRC.search(target.lstrip("./"))
                if not match:
                    continue
                path = target.lstrip("./")
                if path.startswith("../"):
                    resolved = file.parent / path
                else:
                    resolved = ROOT / path
                source = re.sub(r":\d+(?:-\d+)?$", "", resolved.as_posix())
                if not Path(source).exists():
                    findings.append(f"STALE-REF    {rel}  {target}")
    if findings:
        print("\n".join(findings))
        print(f"\ndocs-check: {len(findings)} finding(s) — fix the above or add a stale-ok marker.")
        return 1
    print(f"docs-check: 0 findings across {len(link_files)} link-file(s) and {len(ref_files)} ref-file(s).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
