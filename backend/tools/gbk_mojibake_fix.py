# -*- coding: utf-8 -*-
"""Reverse UTF-8-as-GBK mojibake: encode garbled CJK as GBK bytes, decode as UTF-8."""
from __future__ import annotations

import os
import re
import sys


def fix_cjk_chunk(chunk: str) -> str:
    if not chunk:
        return chunk
    try:
        b = chunk.encode("gbk")
    except UnicodeEncodeError:
        return chunk
    try:
        fixed = b.decode("utf-8")
    except UnicodeDecodeError:
        return chunk
    if "\ufffd" in fixed:
        return chunk
    # Avoid flipping already-valid Chinese that happens to round-trip wrong
    if fixed == chunk:
        return chunk
    return fixed


def fix_line(line: str) -> str:
    out: list[str] = []
    i = 0
    n = len(line)
    while i < n:
        o = ord(line[i])
        if o < 128:
            j = i + 1
            while j < n and ord(line[j]) < 128:
                j += 1
            out.append(line[i:j])
            i = j
            continue
        j = i + 1
        while j < n and ord(line[j]) >= 128:
            j += 1
        chunk = line[i:j]
        out.append(fix_cjk_chunk(chunk))
        i = j
    return "".join(out)


def should_skip_path(path: str) -> bool:
    p = path.replace("\\", "/")
    if "/vendor/" in p:
        return True
    return False


def process_file(path: str) -> bool:
    try:
        with open(path, "r", encoding="utf-8") as f:
            text = f.read()
    except (OSError, UnicodeDecodeError):
        return False
    if "\x00" in text:
        return False
    lines = text.splitlines(keepends=True)
    new_lines = [fix_line(L) for L in lines]
    new_text = "".join(new_lines)
    if new_text == text:
        return False
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(new_text)
    return True


def main() -> int:
    root = os.path.normpath(os.path.join(os.path.dirname(__file__), ".."))
    exts = {".go"}
    changed = 0
    for dirpath, _, filenames in os.walk(root):
        for fn in filenames:
            ext = os.path.splitext(fn)[1].lower()
            if ext not in exts:
                continue
            path = os.path.join(dirpath, fn)
            if should_skip_path(path):
                continue
            if "fix_mojibake" in path or "gbk_mojibake_fix" in path:
                continue
            if process_file(path):
                changed += 1
                print(path)
    print("changed files:", changed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
