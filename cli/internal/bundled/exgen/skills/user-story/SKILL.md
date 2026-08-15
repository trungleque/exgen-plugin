---
name: user-story
description: Turn a requirement, feature, or user story into a sequence of reviewable structured prompts, backed by deep codebase exploration, explicit clarifying questions, and architecture options. Use whenever the user wants to plan a feature end-to-end, break a story or epic into implementable tasks, or asks to "run the story workflow" / "decompose this feature" / "plan and generate prompts" for a requirement. For a single small feature that needs just one prompt without exploration, use the structured-prompt skill directly instead.
argument-hint: "[feature or requirement description, optionally with a ticket id]"
disable-model-invocation: true
---

# User Story

You are helping a developer implement a new feature. Follow a systematic approach: understand the codebase deeply, identify and ask about all underspecified details, design elegant architectures, decompose the requirement into the **smallest independent tasks that still deliver something checkable**, then produce one structured prompt per task using the `structured-prompt` skill.

The output of this workflow is a set of files under `spdd/prompt/` that a human can review in order — the full implementation plan, checkable before any code is generated.

## Core Principles

- **Ask clarifying questions**: Identify all ambiguities, edge cases, and underspecified behaviors. Ask specific, concrete questions rather than making assumptions. Wait for user answers before proceeding. Ask questions early — after understanding the codebase, before designing architecture — because that is when answers are cheapest to incorporate.
- **Understand before acting**: Read and comprehend existing code patterns first.
- **Read files identified by agents**: When launching agents, ask them to return lists of the most important files to read. After agents complete, read those files yourself to build detailed context — agent summaries are a map, not the territory.
- **Simple and elegant**: Prioritize readable, maintainable, architecturally sound designs.
- **Decomposition**: The smallest independent tasks that still deliver something checkable.
- **One prompt per task**: Every task ends up as a structured prompt a human can verify in minutes.

---

## Phase 1: Discovery

**Goal**: Understand what needs to be built

Initial request: $ARGUMENTS

**Actions**:
1. Create a todo list with all six phases
2. If the feature is unclear, ask the user:
   - What problem are they solving?
   - What should the feature do?
   - Any constraints or requirements?
3. Note whether a ticket id (e.g. JIRA-123) was provided — it drives file naming in Phase 6
4. Summarize your understanding and confirm with the user

---

## Phase 2: Codebase exploration

**Goal**: Understand relevant existing code and patterns at both high and low levels

If there is no codebase (greenfield work), say so, skip this phase, and carry that fact forward — the structured prompts in Phase 6 will note it.

**Actions**:
1. Launch 2-3 `code-explorer` agents in parallel (if that agent type is not available, use general-purpose subagents with the same prompts). Each agent should:
   - Trace through the code comprehensively, focusing on abstractions, architecture, and flow of control
   - Target a different aspect (similar features, high-level architecture, the affected area, UI/testing patterns)
   - Return a list of 5-10 key files to read

   **Example agent prompts**:
   - "Find features similar to [feature] and trace through their implementation comprehensively"
   - "Map the architecture and abstractions for [feature area], tracing through the code comprehensively"
   - "Analyze the current implementation of [existing feature/area], tracing through the code comprehensively"
   - "Identify UI patterns, testing approaches, or extension points relevant to [feature]"

2. Read all files the agents identified to build deep understanding
3. Present a comprehensive summary of findings and patterns discovered — this summary is the **codebase analysis** that Phase 6 hands to the structured-prompt skill, so make it self-contained: name the key entities, services, conventions, and file paths

---

## Phase 3: Clarifying Questions

**Goal**: Fill in gaps and resolve all ambiguities before designing

**CRITICAL**: This is one of the most important phases. DO NOT SKIP. Every ambiguity resolved here is one fewer `(ASSUMPTION)` marker polluting the structured prompts later — the prompts should encode decisions, not guesses.

**Actions**:
1. Review the codebase findings and the original feature request
2. Identify underspecified aspects: edge cases, error handling, integration points, scope boundaries, design preferences, backward compatibility, performance needs
3. Present all questions to the user in a clear, organized list
4. Wait for answers before proceeding to architecture design

If the user says "whatever you think is best", provide your recommendation and get explicit confirmation. Record every answer — they become Scope and Safeguard lines in Phase 6.

---

## Phase 4: Architecture Design

**Goal**: Design multiple implementation approaches with different trade-offs

**Actions**:
1. Launch 2-3 `code-architect` agents in parallel with different focuses: minimal changes (smallest change, maximum reuse), clean architecture (maintainability, elegant abstractions), pragmatic balance (speed + quality). Fall back to general-purpose subagents if the agent type is unavailable.
2. Review all approaches and form your own opinion on which fits best for this specific task (consider: small fix vs large feature, urgency, complexity, team context)
3. Present to the user: a brief summary of each approach, a trade-off comparison, your recommendation with reasoning, and the concrete implementation differences
4. **Ask the user which approach they prefer** and record the decision — the chosen approach becomes the Approach section backbone of every structured prompt

---

## Phase 5: Decompose into the smallest independent tasks

**Goal**: Split the chosen approach into an ordered list of tasks, each of which will become one structured prompt

**What makes a good task**:
- **Independently checkable**: when the task is done, something observable works — a passing test, a callable endpoint, a visible UI state. "Create the models" plus "wire everything up" fails this; each slice should prove itself.
- **Vertical over horizontal** where possible: prefer a thin end-to-end slice (one field, one path, one happy case) over a layer-by-layer split, because vertical slices are checkable and surface integration problems in task 1 instead of task N.
- **One sitting of work**: small enough to implement in a single focused session and review as a single diff. If describing the task needs more than a couple of Operations-style steps at this level, split it.
- **Minimal coupling, explicit dependencies**: tasks should be as independent as possible; where a task genuinely needs an earlier one, that dependency must be stated, never implied.

**Actions**:
1. Derive the task list from the chosen architecture. Order it by dependency — the order will become the prompt file order.
2. For each task, write one line of goal and one line of "checkable when": how a reviewer verifies the task is done.
3. Sanity-check the split: could any two tasks merge without losing checkability? Could any task not be verified on its own? Fix before presenting.
4. Present the numbered task list (goal + checkable-when + dependencies) to the user and **get confirmation before generating prompts**. This list is cheap to reorder now and expensive to reorder once six prompt files exist.

A feature small enough to be one task is a valid outcome — the workflow then produces a single structured prompt.

---

## Phase 6: Generate a structured prompt for each task

**Goal**: One reviewable prompt file per task, in execution order, grounded in everything learned in Phases 2-4

**Actions**:
1. For each task, in dependency order, **use the `structured-prompt` skill** (this plugin: `/exgen:structured-prompt`) with:
   - The task's goal and checkable-when line as the feature description
   - The Phase 2 codebase analysis as its existing analysis — the skill's own instructions say to reuse a prior analysis and not re-scan; hold it to that. Re-scanning the codebase per task wastes context and can produce prompts inconsistent with each other.
   - The Phase 4 chosen approach: each prompt's Approach section must be consistent with the story-level decision, scoped down to its task
   - The Phase 3 answers: encode them as Scope In/Out items and Safeguards, not as assumptions
2. **Make each prompt self-contained.** A task may be implemented in a fresh session that has never seen this conversation. Dependencies on earlier tasks go into the prompt explicitly — e.g. Scope In: "Uses the `RateLimiter` introduced in 001", Safeguards: "Do not modify the migration from 002."
3. **File naming** follows the structured-prompt skill's rules, with one story-level addition: when a single ticket covers multiple tasks, suffix a task index to keep names unique and ordered — `JIRA-123-1-add-refund-amount-column.md`, `JIRA-123-2-partial-refund-service.md`. Without a ticket, the skill's own sequence numbering (`001-`, `002-`, ...) already encodes the order.
4. Assumptions in generated prompts should be rare by this point — Phase 3 exists to eliminate them. If decomposition surfaces a genuinely new ambiguity, follow the structured-prompt skill's rule (assume and flag), and raise it in the final summary.

**Wrap-up** — after all prompts are written, present to the user:
- The ordered list of generated files with each task's one-line goal
- Any assumptions or risks that survived into the prompts, so they know exactly what to double-check
- A reminder that the prompts are the review surface: read them in order, correct anything wrong, and only then start code generation

Do not paste the prompts' contents into chat — the summary and file list are the deliverable here, same as in the structured-prompt skill.
