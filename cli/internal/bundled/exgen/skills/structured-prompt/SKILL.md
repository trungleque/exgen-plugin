---
name: structured-prompt
description: Generate a lightweight structured prompt (a simplified SPDD / REASONS-style canvas) from a feature or requirement description — a single short markdown file a human can review in a couple of minutes before any code is generated. Use this skill whenever the user wants a structured prompt, an SPDD prompt or REASONS canvas, a reviewable implementation blueprint, or asks to "spec out" / "plan" / "write a prompt for" a feature before coding. Prefer this over full spec-driven-development workflows when the user wants ONE concise reviewable file rather than a multi-document spec suite.
---

# Structured Prompt

Turn a feature or requirement description into a **structured prompt**: a single, reviewable artifact that captures intent, design, and execution boundaries *before* any code is generated. It is a deliberately trimmed version of the full SPDD REASONS Canvas — five sections instead of seven — small enough that a human can read and verify the whole thing in a couple of minutes.

## Why this exists

A long, exhaustive spec looks thorough but is hard to actually verify — reviewers skim and drift creeps in. This skill optimizes for the opposite: the prompt must be clean, clear and short enough that a person can hold the entire plan in their head and check *intent*, not just completeness. Brevity is a feature, not a limitation. If a section is getting long, that is a signal to sharpen the thinking, not to expand the budget.

**Length budget: the finished prompt should land between 40 and 120 lines.** Under 40 usually means the Operations are too vague to check; over 120 means it can no longer be reviewed in one sitting and needs to be cut or the feature split.

## Workflow

The only required input is a **feature or requirement description** — a paragraph, a few bullets, or a short user story. A JIRA ticket id may accompany it (used in the filename, see Output).

### 1. Reuse existing analysis, or scan the relevant code

If the conversation already contains an analysis of the codebase — for example produced earlier in this session or by another exgen command — **reuse it and do not re-scan**. Re-reading files you already understand wastes context and time.

Otherwise, if a codebase is available (files present, a repo path, or uploaded source): extract the key nouns and verbs from the requirement description (entity names, feature terms, affected modules) and search **only for those** — do not read everything. Look for:

- Existing entities, models, or types the feature touches
- The service/controller/handler the change belongs in
- Established patterns to stay consistent with (naming, layering, error handling)

Ground the Entities and Approach sections in what actually exists. If there is **no codebase**, work from the description alone and say so in one line at the top of the Approach section (e.g. "No codebase available; approach assumes a standard layered service").

### 2. Write the structured prompt

Fill in the five-section template below. Use plain language. Prefer concrete, testable statements over abstract goals.

If a business rule is ambiguous, do not guess silently and do not stall the workflow with questions: write your best assumption into the relevant section, and flag it in the final summary so the user can correct it cheaply — before code exists. Only stop to ask when the ambiguity is so fundamental that the Approach itself would change (e.g. it's unclear whether the feature is per-user or per-account).

### 3. Save and summarize

Save the prompt as a markdown file (see Output). Then give the user a **3–5 sentence summary** in chat: what the feature is, the chosen approach, and anything that needs their confirmation — assumptions you made, Scope In items you added that they didn't explicitly ask for, and risky bits. Point them to the file. Do **not** paste the full prompt into chat — the point is a reviewable file; the summary tells them what to look at.

## The template

Use this exact structure. Headings stay; fill the content.

```markdown
# [Feature name]

## Requirements
**Scope In**
- [What this change WILL do — each item concrete and verifiable]

**Scope Out**
- [What this change explicitly will NOT do — defer/exclude, prevents scope creep]

## Entities
[Domain model in plain text. List each entity, its key fields, and its
relationships in prose or bullets. Mark which are NEW vs EXISTING. No diagrams.]

## Approach
[The chosen strategy in 3-8 lines. Name the design pattern or solution shape
(e.g. Strategy, Factory), the key decisions, and the trade-off accepted. This is
the "how we'll solve it," not the step-by-step.]

## Operations
[Ordered, testable task breakdown. Each step is one concrete unit of work,
specific enough to implement and check. Reference classes/methods when grounded
in a codebase. Number them so review and code generation follow the same order.]
1. [Step]
2. [Step]

## Safeguards
- [Non-negotiable constraints: invariants, validation rules, security limits,
  performance bounds, backward-compatibility requirements. What must NOT break.]
```

## Section guidance

**Requirements (Scope In / Scope Out)** — Scope Out is as important as Scope In; it is the cheapest place to prevent the model from over-building. If the description implies boundaries ("just the calculation, not invoicing"), make them explicit here.

**Entities** — Plain text only, no Mermaid or diagrams. State each entity, the fields that matter for *this* change (not the whole schema), and how they relate. Distinguish NEW concepts from EXISTING ones so the reviewer knows what is being introduced.

**Approach** — This is where intent alignment lives. Name the actual strategy and the reason for it. If you considered an alternative and rejected it, one line on why earns its place. Keep it to the shape of the solution; details go in Operations.

**Operations** — The bridge from idea to code. Each step should be independently checkable. Order matters: the steps are the sequence code generation will follow. Be specific enough that a reviewer can spot a wrong step *before* code exists — down to method signatures when the codebase gives you that grounding.

**Safeguards** — The hard boundaries. Validation rules, security constraints, performance limits, invariants, things that must stay backward-compatible. These are non-negotiable; if violated, the implementation is wrong regardless of whether tests pass.

## Calibration example

This is the level of terseness to aim for. Note how every Scope In item is checkable, the Approach fits in four lines, and each Operation is one unit of work:

```markdown
# Rate-limit password reset requests

## Requirements
**Scope In**
- Limit password-reset emails to 3 per email address per hour
- Return the same generic 200 response whether limited or not (no enumeration)
- Log limited attempts with email hash and timestamp

**Scope Out**
- No CAPTCHA integration
- No changes to the reset-token flow itself
- No per-IP limiting (tracked separately in AUTH-88)

## Entities
- ResetAttempt (NEW): emailHash, requestedAt. No relations; pruned after 24h.
- User (EXISTING): untouched — limiting keys off the submitted email, not the account.

## Approach
Sliding-window counter in Redis keyed by SHA-256 of the normalized email,
checked in AuthService.requestPasswordReset before the existing send path.
Chose Redis TTL over a DB table to avoid a pruning job; trade-off is that
limits reset on cache flush, which is acceptable for this threat model.

## Operations
1. Add RateLimiter.checkAndIncrement(key, limit=3, windowSecs=3600) wrapping Redis INCR + EXPIRE
2. Call it at the top of AuthService.requestPasswordReset; on limit, skip send, still return 200
3. Emit auth.reset_limited log event with emailHash
4. Unit tests: under limit sends, at limit skips, window expiry re-allows

## Safeguards
- Response body and status must be identical in limited and unlimited cases
- Never log or store the raw email in the limiter path — hash only
- If Redis is unavailable, fail open (send the email) and log a warning
```

## Output

Save the prompt under **`spdd/prompt/`** (create the directory if it does not exist).

**File naming** — `[id]-[kebab-case-feature-name].md`, where `[id]` is determined as follows:

- **JIRA ticket provided** → use the ticket as the id, e.g.
  `JIRA-123-password-reset-via-email.md`. The ticket replaces the sequence
  number entirely; do not also add a sequence number.
- **No ticket** → use a zero-padded three-digit sequence number starting at
  `001`. Determine the next number by scanning `spdd/prompt/` for existing files
  whose names start with three digits, taking the highest, and adding one.
  Example progression: `001-password-reset-via-email.md`, then `002-sso-login.md`.

A reliable way to compute the next sequence number (handles an empty directory
and avoids the leading-zero/octal trap on values like `008`/`009`):

```bash
last=$(ls spdd/prompt/ 2>/dev/null | grep -E '^[0-9]{3}-' | sort | tail -1)
num=$(echo "$last" | grep -oE '^[0-9]{3}' | sed 's/^0*//')
printf '%03d\n' $(( ${num:-0} + 1 ))   # empty dir -> 001
```

Present the file to the user, then give the short summary described in step 3.

## Keeping intent reviewable

- Every Scope In item should be traceable to something the user asked for. If you added it, flag it in the summary.
- Assumptions live in the document *and* in the summary — the document so code generation respects them, the summary so the human notices them.
