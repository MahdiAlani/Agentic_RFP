"""Offline eval harness: replay a fixed question set and grade the answers.

Uploads the demo knowledge base into a fresh workspace on a running stack,
asks every question in cases.json through the normal answer pipeline, then
grades each answer:

  - fact recall   an LLM checks which expected facts the answer states
  - supported     the quality checks confirm every claim and figure is backed
                  by a cited passage (see src/quality.py)
  - abstention    questions the documents can't answer must cite nothing

Exits non-zero when a metric falls below its threshold, so it can gate a
deploy. Calls OpenAI for drafting and grading (a few cents per run).

Run:  docker compose run --rm eval
      cd workers && python -m eval.run [--keep] [--min-recall 0.8 ...]
"""

import argparse
import json
import logging
import os
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict, dataclass, field
from pathlib import Path

from src.api_client import ApiClient, wait_for
from src.config import OPENAI_JUDGE_MODEL
from src.llm import chat
from src.quality import Draft, blockers, run_checks

HERE = Path(__file__).resolve().parent
API_URL = os.getenv("API_URL", "http://localhost:8080")
DEMO_DIR = Path(os.getenv("DEMO_DIR", HERE.parent.parent / "demo"))
EVAL_EMAIL = os.getenv("EVAL_EMAIL", "eval@corvane.example")
EVAL_PASSWORD = os.getenv("EVAL_PASSWORD", "corvane-eval")

# Checks that together mean "everything this answer says is backed by its sources".
SUPPORT_CHECKS = {"grounded", "numbers_grounded", "citation_markers_valid"}

DEFAULT_THRESHOLDS = {"fact_recall": 0.8, "supported": 0.9, "abstention": 0.75}

RECALL_PROMPT = """Does the ANSWER state each EXPECTED FACT? Paraphrases count; contradictions do not.

Reply with a JSON object: {{"facts": [{{"fact": "<expected fact>", "present": true or false}}, ...]}}

QUESTION: {question}

ANSWER:
{answer}

EXPECTED FACTS:
{facts}"""

logging.basicConfig(level=logging.INFO, format="%(message)s")
logging.getLogger("httpx").setLevel(logging.WARNING)  # one line per request is noise here
log = logging.getLogger("eval")


@dataclass
class CaseResult:
    id: str
    question: str
    answerable: bool
    answer: str
    citations: int
    recall: float | None = None          # answerable cases only
    missing_facts: list[str] = field(default_factory=list)
    supported: bool | None = None        # answerable cases only
    abstained: bool | None = None        # unanswerable cases only
    failed_checks: dict[str, str] = field(default_factory=dict)  # blocker name -> reason

    @property
    def passed(self) -> bool:
        if not self.answerable:
            return bool(self.abstained)
        return self.recall == 1.0 and bool(self.supported)


# --- grading ------------------------------------------------------------------

def fact_recall(question: str, answer: str, facts: list[str]) -> tuple[float, list[str]]:
    """Fraction of expected facts the answer states, plus the ones it missed."""
    prompt = RECALL_PROMPT.format(question=question, answer=answer,
                                  facts="\n".join(f"- {f}" for f in facts))
    verdicts = json.loads(chat([{"role": "user", "content": prompt}], temperature=0, json_mode=True,
                               model=OPENAI_JUDGE_MODEL))
    present = {v.get("fact"): v.get("present") is True for v in verdicts.get("facts", [])}
    missing = [f for f in facts if not present.get(f)]
    return 1 - len(missing) / len(facts), missing


def grade(case: dict, row: dict) -> CaseResult:
    text = row.get("answer_text") or ""
    cites = row.get("citations") or []
    result = CaseResult(
        id=case["id"], question=case["question"], answerable=case.get("answerable", True),
        answer=text, citations=len(cites),
    )

    if not result.answerable:
        # The right response to an uncovered question is to decline, citing nothing.
        result.abstained = not cites
        return result

    checks = run_checks(Draft(question=case["question"], text=text, citations=cites))
    result.failed_checks = {c.name: c.detail for c in blockers(checks)}
    result.supported = not SUPPORT_CHECKS & result.failed_checks.keys()
    result.recall, result.missing_facts = fact_recall(case["question"], text, case["expect"])
    return result


def summarize(results: list[CaseResult]) -> dict[str, float]:
    answerable = [r for r in results if r.answerable]
    unanswerable = [r for r in results if not r.answerable]

    def mean(values):
        values = list(values)
        return sum(values) / len(values) if values else 1.0

    return {
        "fact_recall": mean(r.recall for r in answerable),
        "supported": mean(r.supported for r in answerable),
        "abstention": mean(r.abstained for r in unanswerable),
        "passed": mean(r.passed for r in results),
    }


def threshold_failures(metrics: dict[str, float], thresholds: dict[str, float]) -> list[str]:
    return [
        f"{name} {metrics[name]:.0%} < {minimum:.0%}"
        for name, minimum in thresholds.items()
        if metrics[name] < minimum
    ]


def render_markdown(results: list[CaseResult], metrics: dict, failures: list[str]) -> str:
    lines = [
        "## RFP answer eval",
        "",
        f"**{'FAIL' if failures else 'PASS'}**" + (f": {'; '.join(failures)}" if failures else ""),
        "",
        "| Metric | Score |",
        "|---|---|",
        *(f"| {name} | {value:.0%} |" for name, value in metrics.items()),
        "",
        "| Case | Result | Recall | Supported | Sources | Notes |",
        "|---|---|---|---|---|---|",
    ]
    for r in results:
        if r.answerable:
            notes = "; ".join([*(f"missing: {f}" for f in r.missing_facts),
                               *(f"{name}: {why}" for name, why in r.failed_checks.items())])
            recall, supported = f"{r.recall:.0%}", "yes" if r.supported else "no"
        else:
            notes = "declined correctly" if r.abstained else "answered an uncovered question"
            recall = supported = "-"
        lines.append(f"| {r.id} | {'pass' if r.passed else 'FAIL'} | {recall} | {supported} | {r.citations} | {notes} |")
    return "\n".join(lines) + "\n"


# --- run --------------------------------------------------------------------

def ask_all(api: ApiClient, cases: list[dict], keep: bool = False) -> list[dict]:
    """Load the corpus into a fresh workspace, ask every question, return the answers."""
    workspace = api.post("/workspaces", {"name": f"eval {time.strftime('%Y-%m-%d %H:%M:%S')}"})
    try:
        docs = [api.upload_document(workspace["id"], p) for p in sorted((DEMO_DIR / "knowledge-base").glob("*"))]
        for doc in docs:
            status = wait_for(lambda: api.get(f"/documents/{doc['id']}")["status"],
                              lambda s: s in ("embedded", "failed"))
            if status == "failed":
                raise RuntimeError(f"embedding failed for {doc['file_name']}")
        log.info("embedded %d documents; asking %d questions...", len(docs), len(cases))

        project = api.post("/projects", {"workspace_id": workspace["id"], "name": "eval"})
        ids = [
            api.post("/answers", {"workspace_id": workspace["id"], "project_id": project["id"],
                                  "question": c["question"]})["id"]
            for c in cases
        ]
        wait_for(lambda: api.get(f"/projects/{project['id']}/answers"),
                 lambda rows: all(a["status"] in ("done", "failed") for a in rows), timeout=900)
        return [api.get(f"/answers/{i}") for i in ids]
    finally:
        if not keep:
            api.delete(f"/workspaces/{workspace['id']}")


def parse_args(argv=None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--cases", type=Path, default=HERE / "cases.json")
    p.add_argument("--out", type=Path, default=HERE / "last-run.json", help="where to write the JSON report")
    p.add_argument("--keep", action="store_true", help="keep the eval workspace for inspection")
    for name, default in DEFAULT_THRESHOLDS.items():
        p.add_argument(f"--min-{name.replace('_', '-')}", type=float, default=default, dest=f"min_{name}")
    return p.parse_args(argv)


def main() -> None:
    args = parse_args()
    cases = json.loads(args.cases.read_text())["cases"]

    api = ApiClient(API_URL)
    api.login_or_register(EVAL_EMAIL, "Eval Harness", EVAL_PASSWORD)
    rows = ask_all(api, cases, keep=args.keep)

    log.info("grading...")
    with ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(grade, cases, rows))

    metrics = summarize(results)
    thresholds = {name: getattr(args, f"min_{name}") for name in DEFAULT_THRESHOLDS}
    failures = threshold_failures(metrics, thresholds)
    report = render_markdown(results, metrics, failures)

    print(report)
    args.out.write_text(json.dumps({"metrics": metrics, "failures": failures,
                                    "cases": [asdict(r) for r in results]}, indent=2))
    if summary := os.getenv("GITHUB_STEP_SUMMARY"):
        with open(summary, "a", encoding="utf-8") as f:
            f.write(report)

    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
