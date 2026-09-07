---
name: implementation
description: Implement a feature from a structured prompt under spdd/prompt/ using strict TDD (Red-Green-Refactor, one Operation at a time). Use when asked to implement, build, develop, or start coding a prompt, spec, or task file (e.g. "implement 001", "build next prompt").
argument-hint: "[prompt id or path, e.g. 001, JIRA-123, or spdd/prompt/001-foo.md]"
---

# Implementation

Execute a structured prompt (from `structured-prompt` or `user-story`) as working, tested code. The prompt file is the binding contract: Scope In is the definition of done, Scope Out is a hard boundary, Approach is the approved architecture, Operations are the execution order, and Safeguards are mandatory invariants.

Development is strict TDD: no production code without a failing test first.

## Reference file

**`references/testing-anti-patterns.md`** — read only when creating mocks, stubs, spies, or test doubles, or if tempted to add test-only methods to production code. Do not load for pure logic or mock-free unit tests. When test doubles or isolation are needed, its gate functions are mandatory.

## Workflow

### 1. Locate and read the prompt

Resolve `$ARGUMENTS` to a file in `spdd/prompt/`:
- ID (`001`, `JIRA-123`) → file matching prefix
- Path → that file
- Omitted → lowest-numbered prompt in `spdd/prompt/` (confirm with user first)

Read the prompt and check dependencies: if Scope In or Safeguards reference earlier tasks (e.g. "uses RateLimiter from 001"), verify that code exists. If missing, stop and inform the user which prompt must precede this one.

### 2. Ground prompt in codebase (targeted reading)

Inspect only files implied by the prompt: EXISTING entities, services named in Operations, and neighboring test setups.
- **Prune context:** Use targeted searches (`grep_search`) or line-bounded reads (`StartLine`/`EndLine`) for signatures/types. Do not view entire large files or scan unrelated modules.
- **Contradiction protocol:** If reality contradicts the prompt (missing entity, different signature, impossible operation), **stop. Do not improvise.** Propose the minimal prompt amendment and get user confirmation.

### 3. Plan the test map

Map what proves completion:
- Each **Operation** → driving test(s)
- Each **Safeguard** → at least one test that fails on violation (or explain if untestable)
- Each **Scope In** item → observable proof / test

Keep this map as your checklist for the completion report.

### 4. TDD loop — one Operation at a time

Execute Operations strictly in numbered order. For prompts with 6+ operations, consider delegating subsets to isolated subagents to keep context compact.

For each Operation:
1. **Red** — write the failing test. Run **only this specific test** (targeted test runner command); verify it fails for missing behavior, not setup errors.
2. **Green** — write minimal production code to pass. Re-run the targeted test to verify green. No speculative code beyond the current test.
3. **Refactor** — clean up while keeping the targeted test green.
4. **Regression check** — run the package or full test suite with quiet/concise flags (avoid `-v` unless debugging) before advancing to the next Operation.

Loop rules:
- **Scope Out is a wall.** Any need for excluded scope is a contradiction → stop and raise it.
- **Mocks follow gate functions** in `references/testing-anti-patterns.md` (read only when adding doubles): understand side effects first, mock at the lowest boundary, mirror complete schemas, and never assert on mock internals.
- **No test-only methods in production classes.** Test cleanup helpers belong in test utilities.
- Write Safeguard tests alongside the Operation that makes them testable — do not defer them all to the end.

### 5. Verify and report

1. Run full test suite one final time.
2. Output a concise checklist in chat (avoid pasting code diffs or verbose test logs):
   - `- [x] Scope In: <item> → <test/proof>`
   - `- [x] Safeguard: <item> → <test/proof>`
   - Approved deviations or resolved `(ASSUMPTION)` tags.
3. "Implementation complete, ready for testing" is forbidden — tests *are* the implementation proof.

### 6. Hand off to review — the mandatory gate

Confirm report with user, then invoke the review skill (`/exgen:review`).
- The prompt file **remains in `spdd/prompt/`** until review verdict is APPROVE (which moves it to `spdd/done/`). Never move the file yourself.
- On **REQUEST CHANGES**: address blocking findings through the TDD loop (failing test first), then re-run review.

## Failure handling

- Test won't go green after few attempts: prompt may be wrong (step 2 protocol) or understanding is incomplete.
- Never weaken or delete a failing Safeguard test to force progress.
- If suite was red before starting, alert the user immediately; do not proceed without a green baseline.
