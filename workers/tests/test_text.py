import io

import docx
import pytest

from src.text import chunk_text, extract_text


def _pdf_with_text(text: str) -> bytes:
    """Build a minimal one-page PDF that draws `text` with a standard font."""
    stream = f"BT /F1 12 Tf 72 720 Td ({text}) Tj ET".encode()
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        b"/Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
        b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream",
        b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    ]
    out = io.BytesIO(b"%PDF-1.4\n")
    offsets = []
    for i, body in enumerate(objects, start=1):
        offsets.append(out.tell())
        out.write(b"%d 0 obj\n" % i + body + b"\nendobj\n")
    xref = out.tell()
    out.write(b"xref\n0 %d\n0000000000 65535 f \n" % (len(objects) + 1))
    for off in offsets:
        out.write(b"%010d 00000 n \n" % off)
    out.write(b"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (len(objects) + 1, xref))
    return out.getvalue()


def _docx_with(paragraphs: list[str]) -> bytes:
    d = docx.Document()
    for p in paragraphs:
        d.add_paragraph(p)
    buf = io.BytesIO()
    d.save(buf)
    return buf.getvalue()


class TestExtractText:
    def test_plain_text_and_unknown_extensions_decode_as_utf8(self):
        assert extract_text("notes.md", "héllo".encode()) == "héllo"
        assert extract_text("README", b"no extension") == "no extension"

    def test_invalid_utf8_is_replaced_not_raised(self):
        assert "�" in extract_text("x.txt", b"bad \xff byte")

    def test_pdf(self):
        assert "Encryption at rest" in extract_text("spec.PDF", _pdf_with_text("Encryption at rest"))

    def test_docx_skips_blank_paragraphs(self):
        text = extract_text("spec.docx", _docx_with(["First", "", "Second"]))
        assert text == "First\n\nSecond"


class TestChunkText:
    def test_empty_and_whitespace(self):
        assert chunk_text("") == []
        assert chunk_text("  \n\n  ") == []

    def test_short_text_is_one_chunk_with_whitespace_collapsed(self):
        assert chunk_text("Hello   world.\nSame paragraph.") == ["Hello world. Same paragraph."]

    def test_paragraphs_pack_until_size(self):
        paras = ["a" * 40, "b" * 40, "c" * 40]
        chunks = chunk_text("\n\n".join(paras), size=100, overlap=0)
        assert chunks == [f"{paras[0]}\n\n{paras[1]}", paras[2]]

    def test_overlap_repeats_tail_of_previous_chunk(self):
        paras = ["a" * 60, "b" * 60]
        chunks = chunk_text("\n\n".join(paras), size=100, overlap=10)
        assert len(chunks) == 2
        assert chunks[1].startswith("a" * 10)
        assert chunks[1].endswith("b" * 60)

    def test_long_paragraph_splits_on_sentences(self):
        sentences = [f"Sentence number {i} is here." for i in range(20)]
        chunks = chunk_text(" ".join(sentences), size=120, overlap=0)
        assert len(chunks) > 1
        # No sentence is cut in half when sentences fit in a chunk.
        for chunk in chunks:
            assert chunk.endswith(".")
        assert " ".join(chunks).count("Sentence number") == 20

    def test_oversized_sentence_is_hard_wrapped(self):
        chunks = chunk_text("x" * 250, size=100, overlap=0)
        assert [len(c) for c in chunks] == [100, 100, 50]

    @pytest.mark.parametrize("size,overlap", [(200, 0), (200, 30), (500, 50)])
    def test_no_text_is_lost(self, size, overlap):
        words = [f"w{i}" for i in range(400)]
        text = "\n\n".join(" ".join(words[i : i + 25]) + "." for i in range(0, 400, 25))
        joined = " ".join(chunk_text(text, size=size, overlap=overlap))
        for w in words:
            assert w in joined

    def test_chunks_stay_near_size(self):
        text = "\n\n".join("word " * 50 for _ in range(30))
        for chunk in chunk_text(text, size=300, overlap=40):
            assert len(chunk) <= 300 + 40 + 2
