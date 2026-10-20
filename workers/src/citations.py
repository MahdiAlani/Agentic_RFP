"""Inline citation handling for generated answers.

The model is shown numbered context passages and cites them inline as [1],
[2] or [1, 3]. `resolve` keeps only the passages the answer actually cites,
renumbers them 1..n in order of first use, and rewrites the markers to match,
so the stored citations line up with the text the user sees.
"""

import re

# [1], [2, 5], [3,4] - one bracket holding one or more passage numbers.
_MARKER = re.compile(r"\[(\d+(?:\s*,\s*\d+)*)\]")


def resolve(text: str, chunks: list[dict]) -> tuple[str, list[dict]]:
    """Return (text with renumbered markers, cited chunks in marker order).

    Markers pointing outside 1..len(chunks) are model mistakes; they are
    dropped rather than kept as citations to nothing.
    """
    order: dict[int, int] = {}  # original passage index -> new marker number

    def renumber(match: re.Match) -> str:
        numbers = []
        for raw in match.group(1).split(","):
            idx = int(raw) - 1
            if not 0 <= idx < len(chunks):
                continue
            new = order.setdefault(idx, len(order) + 1)
            if new not in numbers:
                numbers.append(new)
        return f"[{', '.join(map(str, numbers))}]" if numbers else ""

    rewritten = _MARKER.sub(renumber, text)
    # Dropping a bad marker can leave "word ." or doubled spaces behind.
    rewritten = re.sub(r"[ \t]+([.,;:])", r"\1", rewritten)
    rewritten = re.sub(r"[ \t]{2,}", " ", rewritten).strip()

    cited = sorted(order, key=order.get)
    return rewritten, [chunks[i] for i in cited]


def markers(text: str) -> set[int]:
    """Every passage number referenced by a marker in `text`."""
    return {int(n) for m in _MARKER.finditer(text) for n in m.group(1).split(",")}
