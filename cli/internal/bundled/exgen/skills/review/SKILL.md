---
name: review
description: Independently verify a finished implementation against its structured prompt — the mandatory gate that moves a prompt from spdd/prompt/ to spdd/done/ on APPROVE. Use whenever the user asks to review, verify, audit, or check an implementation against its prompt or spec — e.g. "review 001", "verify JIRA-123 against the prompt", "check this diff against the spec" — and whenever the implementation skill finishes a task. Produces a per-item verdict with evidence, scope-creep findings, and an APPROVE / REQUEST CHANGES conclusion.
argument-hint: "[prompt id or path, e.g. 001 or JIRA-123; optionally a commit range]"
---

# Review

Verify that an implementation actually satisfies its structured prompt. This is the independent counterpart to the `implementation` skill's self-report: the reviewer trusts **evidence** (code read, tests run, diffs inspected), never claims — not the implementer's completion report, not test names, not comments.

The prompt is the contract; the review checks the delivery against it, section by section.

## Independence rule

If the implementation happened **in this same conversation**, do not review it directly — the context that wrote the code will see what it meant, not what it wrote. Instead, launch a general-purpose subagent (or a code-reviewer agent if this plugin provides one) whose entire input is: this skill's instructions, the prompt file, and the change set. Relay its report. If the implementation came from a different session, review directly.

## Workflow

### 1. Locate the contract and the change set

- Resolve `$ARGUMENTS` to a prompt file: check `spdd/prompt/` first (work awaiting review lives there until it passes), then `spdd/done/` (re-auditing already-approved work). An id like `001` or `JIRA-123` matches the filename prefix.
- Determine the change set, in order of preference: a commit range the user gave → the current branch's diff against the default branch (`git diff main...HEAD` or equivalent) → uncommitted changes. State which you are reviewing; if it's ambiguous or mixes multiple prompts' work, ask.
- Read the entire prompt before reading any code.

### 2. Run the tests yourself

Run the full suite. A red suite ends the review immediately: report the failures, verdict is request-changes. Record the test count — you will compare it against the diff's test files to notice tests that exist but are skipped or never run.

### 3. Audit section by section

For each prompt section, produce a verdict per item — ✅ verified, ❌ violated, or ⚠️ unverifiable — each with one line of evidence (a file:line, a test name you ran, a diff hunk). "Looks fine" is not evidence.

- **Scope In** — for each item, find the observable behavior or test that proves it, and read the test body: does it actually assert the behavior, or just execute the code path? An assertion-free or trivially-true test does not verify anything.
- **Scope Out** — scan the diff for anything that implements excluded scope or touches areas the prompt deferred. Scope creep is a finding even when the code is good.
- **Entities** — check the actual schema/model against the section: fields, relations, NEW vs EXISTING as promised. Migrations are part of this check.
- **Approach** — compare the implemented design to the approved one. A working implementation with a silently different architecture is a ❌: the user reviewed and approved a specific approach, and drift here invalidates that review. Note it even if you think the new design is better — that's the user's call.
- **Operations** — confirm each step exists in the code. Missing steps are findings; extra unrequested work goes under scope creep.
- **Safeguards** — the strictest check, see step 4.

### 4. Safeguard verification (spot-check by mutation)

For each Safeguard, find the test that would fail if it were violated. Then, for the one or two **most critical** safeguards, verify the test has teeth:

1. Ensure the working tree is clean or stashed so you can restore perfectly.
2. Deliberately break the safeguard in production code (invert the boundary check, remove the lock, skip the validation).
3. Run the relevant tests — they **must fail**. A safeguard whose test stays green under mutation is unprotected: ❌ with the mutation described as evidence.
4. **Restore immediately** (`git checkout -- <files>` / unstash) and re-run to confirm green. Never leave a mutation in the tree; never commit during this step.

If the environment makes mutation risky (no git, generated code), skip it and mark those safeguards ⚠️ verified-by-reading-only.

### 5. Test quality pass

Check the diff's tests against the testing anti-patterns reference at `../implementation/references/testing-anti-patterns.md` (read it if available). If the file is not present, apply at minimum these red flags:

- Assertions on mock existence or `*-mock` test IDs
- Test-only methods added to production classes
- Mocks replacing behavior the test depends on, or partial mocks missing fields real responses have
- Mock setup dwarfing test logic; tests that fail when a mock is removed
- New production code with no test driving it (TDD violation — the prompt's Operations should each have one)

### 6. Report, verdict, and gate action

This review is the **mandatory gate** in the exgen pipeline: a prompt only ever moves from `spdd/prompt/` to `spdd/done/` through an APPROVE verdict here. Deliver the report directly in chat (no report file):

- The change set reviewed (range/branch) and suite result
- The per-item verdict table for all six sections, each with evidence
- Findings, each tagged **blocking** (violated Safeguard or Scope item, toothless safeguard test, silent re-architecture) or **non-blocking** (scope creep that's harmless, style drift, ⚠️ unverifiables worth a human glance)
- **Verdict: APPROVE** (all Scope In ✅, all Safeguards ✅, no blocking findings) or **REQUEST CHANGES** (anything blocking — list exactly what must change, referencing prompt items by name)

Then act on the verdict:

- **APPROVE** → move the prompt file from `spdd/prompt/` to `spdd/done/` (create the directory if needed, keep the filename). This is the only place in the pipeline that performs this move.
- **REQUEST CHANGES** → leave the prompt where it is. The fix loop belongs to the `implementation` skill with the blocking findings as added context; re-run this review after the fixes.

The review never fixes the code itself — it produces findings and moves (or doesn't move) the file. Nothing else.
