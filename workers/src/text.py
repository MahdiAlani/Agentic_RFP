"""Text extraction and chunking for uploaded documents."""

import io
import logging
import re

from src.config import CHUNK_OVERLAP, CHUNK_SIZE

log = logging.getLogger("text")


def extract_text(file_name: str, data: bytes) -> str:
    """Pull plain text out of an uploaded file, dispatching on its extension."""
    ext = file_name.rsplit(".", 1)[-1].lower() if "." in file_name else ""

    if ext == "pdf":
        return _extract_pdf(data)
    if ext == "docx":
        return _extract_docx(data)
    # txt, md, csv, json and anything else can be decoded as utf8
    return data.decode("utf-8", errors="replace")


def _extract_pdf(data: bytes) -> str:
    from pypdf import PdfReader

    reader = PdfReader(io.BytesIO(data))
    pages = [(page.extract_text() or "") for page in reader.pages]
    return "\n\n".join(pages)


def _extract_docx(data: bytes) -> str:
    import docx

    document = docx.Document(io.BytesIO(data))
    return "\n\n".join(p.text for p in document.paragraphs if p.text.strip())


def chunk_text(
    text: str, size: int = CHUNK_SIZE, overlap: int = CHUNK_OVERLAP
) -> list[str]:
    """Split text into overlapping chunks, preferring paragraph boundaries.

    Paragraphs are packed together until they would exceed `size`; paragraphs
    longer than `size` are split on sentence boundaries first, then hard-wrapped.
    Each new chunk repeats the trailing overlap characters of the previous one
    so context isn't lost.
    """
    text = text.strip()
    if not text:
        return []

    pieces: list[str] = []
    for para in re.split(r"\n\s*\n", text):
        para = " ".join(para.split())  # collapse whitespace
        if not para:
            continue
        pieces.extend(_split_long(para, size) if len(para) > size else [para])

    chunks: list[str] = []
    current = ""
    for piece in pieces:
        if current and len(current) + len(piece) + 2 > size:
            chunks.append(current)
            tail = current[-overlap:] if overlap else ""
            current = f"{tail}\n\n{piece}".strip() if tail else piece
        else:
            current = f"{current}\n\n{piece}" if current else piece
    if current:
        chunks.append(current)

    return chunks


def _split_long(paragraph: str, size: int) -> list[str]:
    """Break an oversized paragraph on sentence boundaries, hard-wrapping if needed."""
    out: list[str] = []
    current = ""
    for sentence in re.split(r"(?<=[.!?])\s+", paragraph):
        while len(sentence) > size:  # a single sentence longer than a chunk
            out.append(sentence[:size])
            sentence = sentence[size:]
        if current and len(current) + len(sentence) + 1 > size:
            out.append(current)
            current = sentence
        else:
            current = f"{current} {sentence}".strip()
    if current:
        out.append(current)
    return out
