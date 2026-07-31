import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("ocr_math_materials.py")


def load_script_module():
    spec = importlib.util.spec_from_file_location("ocr_math_materials", SCRIPT_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class OCRMathMaterialsTests(unittest.TestCase):
    def test_selects_only_found_materials_of_requested_kind(self):
        module = load_script_module()
        manifest = {
            "entries": [
                {"target_id": "book", "kind": "textbook", "status": "found", "path": "/book.pdf"},
                {"target_id": "exam", "kind": "exam", "status": "found", "path": "/exam.pdf"},
                {"target_id": "missing", "kind": "exam", "status": "missing"},
            ]
        }

        self.assertEqual(
            [entry["target_id"] for entry in module.select_entries(manifest, "exam")],
            ["exam"],
        )

    def test_parses_pdfinfo_page_count(self):
        module = load_script_module()
        self.assertEqual(module.parse_pdfinfo_pages("Title: Test\nPages:          123\n"), 123)

    def test_consolidates_resumable_page_text_with_source_markers(self):
        module = load_script_module()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            pages = root / "pages"
            pages.mkdir()
            (pages / "page-0001.txt").write_text("第一题", encoding="utf-8")
            (pages / "page-0002.txt").write_text("第二题", encoding="utf-8")
            output = root / "exam.md"
            module.consolidate_pages(
                {"target_id": "exam", "title": "试卷", "path": "/exam.pdf"},
                pages,
                output,
                2,
            )

            text = output.read_text(encoding="utf-8")
            self.assertIn("# 试卷", text)
            self.assertIn("原始文件：`/exam.pdf`", text)
            self.assertIn("## PDF 第 1 页", text)
            self.assertIn("第二题", text)


if __name__ == "__main__":
    unittest.main()
