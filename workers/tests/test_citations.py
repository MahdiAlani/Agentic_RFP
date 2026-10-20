import pytest

from src.citations import markers, resolve

CHUNKS = [{"id": i} for i in range(1, 6)]  # passages [1]..[5]


def ids(chunks):
    return [c["id"] for c in chunks]


def test_keeps_only_cited_passages_renumbered_by_first_use():
    text, cited = resolve("Data is encrypted [4]. Keys rotate yearly [2]. Again [4].", CHUNKS)
    assert text == "Data is encrypted [1]. Keys rotate yearly [2]. Again [1]."
    assert ids(cited) == [4, 2]


def test_grouped_markers():
    text, cited = resolve("Both apply [3, 1].", CHUNKS)
    assert text == "Both apply [1, 2]."
    assert ids(cited) == [3, 1]


def test_duplicate_numbers_in_one_group_collapse():
    text, _ = resolve("Twice [2,2].", CHUNKS)
    assert text == "Twice [1]."


def test_out_of_range_markers_are_dropped():
    text, cited = resolve("Real [1]. Invented [9]. Mixed [9, 2].", CHUNKS)
    assert text == "Real [1]. Invented. Mixed [2]."
    assert ids(cited) == [1, 2]


def test_zero_is_out_of_range():
    text, cited = resolve("Bad [0].", CHUNKS)
    assert text == "Bad." and cited == []


def test_no_markers_means_no_citations():
    text, cited = resolve("The documentation does not cover this.", CHUNKS)
    assert text == "The documentation does not cover this."
    assert cited == []


def test_preserves_paragraphs():
    text, _ = resolve("First [1].\n\nSecond [2].", CHUNKS)
    assert text == "First [1].\n\nSecond [2]."


@pytest.mark.parametrize("text,expected", [
    ("a [1] b [2, 3]", {1, 2, 3}),
    ("none here", set()),
    ("see [10]", {10}),
])
def test_markers(text, expected):
    assert markers(text) == expected
