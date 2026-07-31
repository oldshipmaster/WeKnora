#!/usr/bin/env python3
"""Extract local textbook PDFs to untracked Markdown for direct WeKnora indexing."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from pypdf import PdfReader


MAX_MANUAL_CHARS = 195_000


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    manifest = json.loads(Path(args.manifest).read_text(encoding="utf-8"))
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    extracted = 0
    for entry in manifest["entries"]:
        if entry["kind"] != "textbook" or entry["status"] != "found":
            continue
        pages = []
        for page_number, page in enumerate(PdfReader(entry["path"]).pages, start=1):
            text = (page.extract_text() or "").replace("\x00", "").strip()
            if text:
                pages.append(f"## 第 {page_number} 页\n\n{text}")
        content = "\n\n".join(
            [
                f"# {entry['title']}",
                f"版本：{entry['edition']}",
                f"年级：{entry['grade']}；学期：{entry['term']}",
                f"本地来源：{entry['path']}",
                *pages,
            ]
        )
        if len(content) > MAX_MANUAL_CHARS:
            raise RuntimeError(
                f"{entry['target_id']} has {len(content)} characters; split it before import"
            )
        (out_dir / f"{entry['target_id']}.md").write_text(content + "\n", encoding="utf-8")
        extracted += 1

    print(json.dumps({"extracted_textbooks": extracted, "output": str(out_dir)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
