#!/usr/bin/env python3
"""OCR found math materials into resumable, page-addressable Markdown."""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path


def select_entries(manifest: dict, kind: str) -> list[dict]:
    entries = []
    for entry in manifest.get("entries", []):
        if entry.get("status") != "found" or not entry.get("path"):
            continue
        if kind != "all" and entry.get("kind") != kind:
            continue
        entries.append(entry)
    return entries


def parse_pdfinfo_pages(output: str) -> int:
    match = re.search(r"(?m)^Pages:\s*(\d+)\s*$", output)
    if not match:
        raise ValueError("pdfinfo output did not contain a page count")
    return int(match.group(1))


def page_count(pdfinfo: str, pdf_path: Path) -> int:
    result = subprocess.run(
        [pdfinfo, str(pdf_path)],
        check=True,
        capture_output=True,
        text=True,
        timeout=120,
    )
    return parse_pdfinfo_pages(result.stdout)


def consolidate_pages(entry: dict, pages_dir: Path, output_path: Path, total_pages: int) -> None:
    sections = [
        f"# {entry['title']}",
        "",
        f"- 目标标识：`{entry['target_id']}`",
        f"- 原始文件：`{entry['path']}`",
        "- 处理方式：本机 Tesseract OCR；页码对应原 PDF，图形与复杂公式需回看原页。",
        "",
    ]
    for page_number in range(1, total_pages + 1):
        page_path = pages_dir / f"page-{page_number:04d}.txt"
        if not page_path.exists():
            raise FileNotFoundError(f"missing OCR page output: {page_path}")
        text = page_path.read_text(encoding="utf-8").strip()
        sections.extend([f"## PDF 第 {page_number} 页", "", text or "[本页未识别到文字]", ""])

    output_path.parent.mkdir(parents=True, exist_ok=True)
    temporary = output_path.with_suffix(output_path.suffix + ".tmp")
    temporary.write_text("\n".join(sections).rstrip() + "\n", encoding="utf-8")
    temporary.replace(output_path)


def resolve_executable(value: str) -> str:
    resolved = shutil.which(value)
    if resolved is None:
        raise FileNotFoundError(f"required executable not found: {value}")
    return resolved


def ocr_page(
    entry: dict,
    page_number: int,
    pages_dir: Path,
    temporary_dir: Path,
    *,
    pdftoppm: str,
    tesseract: str,
    dpi: int,
    language: str,
    psm: int,
    force: bool,
    timeout: int,
) -> tuple[int, int, bool]:
    output_path = pages_dir / f"page-{page_number:04d}.txt"
    if not force and output_path.exists() and output_path.stat().st_size > 0:
        return page_number, output_path.stat().st_size, True

    image_base = temporary_dir / f"{entry['target_id']}-{page_number:04d}"
    image_path = image_base.with_suffix(".png")
    try:
        subprocess.run(
            [
                pdftoppm,
                "-f",
                str(page_number),
                "-l",
                str(page_number),
                "-r",
                str(dpi),
                "-png",
                "-singlefile",
                entry["path"],
                str(image_base),
            ],
            check=True,
            capture_output=True,
            timeout=timeout,
        )
        result = subprocess.run(
            [tesseract, str(image_path), "stdout", "-l", language, "--psm", str(psm)],
            check=True,
            capture_output=True,
            timeout=timeout,
        )
        text = result.stdout.decode("utf-8", errors="replace").strip()
        pages_dir.mkdir(parents=True, exist_ok=True)
        temporary_text = output_path.with_suffix(".txt.tmp")
        temporary_text.write_text(text + "\n", encoding="utf-8")
        temporary_text.replace(output_path)
        return page_number, len(text), False
    finally:
        image_path.unlink(missing_ok=True)


def ocr_entry(entry: dict, args: argparse.Namespace) -> tuple[int, int, int]:
    pdf_path = Path(entry["path"])
    if not pdf_path.is_file():
        raise FileNotFoundError(f"material file is missing: {pdf_path}")
    total_pages = page_count(args.pdfinfo, pdf_path)
    pages_dir = args.output_dir / ".pages" / entry["target_id"]
    temporary_dir = args.output_dir / ".tmp"
    pages_dir.mkdir(parents=True, exist_ok=True)
    temporary_dir.mkdir(parents=True, exist_ok=True)

    completed = 0
    skipped = 0
    characters = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures = [
            executor.submit(
                ocr_page,
                entry,
                page_number,
                pages_dir,
                temporary_dir,
                pdftoppm=args.pdftoppm,
                tesseract=args.tesseract,
                dpi=args.dpi,
                language=args.language,
                psm=args.psm,
                force=args.force,
                timeout=args.page_timeout,
            )
            for page_number in range(1, total_pages + 1)
        ]
        for future in concurrent.futures.as_completed(futures):
            _, page_characters, was_skipped = future.result()
            characters += page_characters
            skipped += int(was_skipped)
            completed += 1
            if completed % 10 == 0 or completed == total_pages:
                print(
                    f"{entry['target_id']}: {completed}/{total_pages} pages, "
                    f"characters={characters}, resumed={skipped}",
                    flush=True,
                )

    consolidate_pages(
        entry,
        pages_dir,
        args.output_dir / f"{entry['target_id']}.md",
        total_pages,
    )
    return total_pages, characters, skipped


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--kind", choices=("exam", "textbook", "all"), default="exam")
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--dpi", type=int, default=180)
    parser.add_argument("--language", default="chi_sim+eng")
    parser.add_argument("--psm", type=int, default=6)
    parser.add_argument("--page-timeout", type=int, default=180)
    parser.add_argument("--pdfinfo", default="pdfinfo")
    parser.add_argument("--pdftoppm", default="pdftoppm")
    parser.add_argument("--tesseract", default="tesseract")
    parser.add_argument("--force", action="store_true")
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.workers < 1:
        raise ValueError("--workers must be at least 1")
    args.pdfinfo = resolve_executable(args.pdfinfo)
    args.pdftoppm = resolve_executable(args.pdftoppm)
    args.tesseract = resolve_executable(args.tesseract)
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    entries = select_entries(manifest, args.kind)
    if not entries:
        raise ValueError(f"manifest has no found {args.kind} materials")

    args.output_dir.mkdir(parents=True, exist_ok=True)
    summary = []
    for entry in entries:
        pages, characters, resumed = ocr_entry(entry, args)
        summary.append(
            {
                "target_id": entry["target_id"],
                "pages": pages,
                "characters": characters,
                "resumed_pages": resumed,
            }
        )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"OCR failed: {error}", file=sys.stderr)
        raise SystemExit(1)
