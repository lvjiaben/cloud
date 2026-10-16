---
name: karpathy-guidelines
description: Behavioral guidelines to reduce common LLM coding mistakes. Use when writing, reviewing, or refactoring code to avoid overcomplication, make surgical changes, surface assumptions, and define verifiable success criteria.
license: MIT
---

# Karpathy Guidelines

Apply these guidelines proportionally to the task. Follow the user's confirmed intent and the repository's decision and validation rules.

## 1. Resolve Material Ambiguity

**Use available evidence and distinguish business decisions from implementation choices.**

Before implementing:
- Check relevant code, requirements, and confirmed decisions before asking.
- Ask only when unresolved alternatives materially change business results, permissions, cost, data ownership, safety boundaries, or the main user workflow. Pause dependent work and continue independent authorized work. Actual tool authorization requirements still apply.
- Choose routine internal names, code organization, and equivalent implementations using project conventions; explain a tradeoff only when it helps the user assess the result.
- Report conclusions, key evidence, and material assumptions concisely, following the project's communication preferences.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- Prefer the simplest implementation that preserves required behavior, error handling, and maintainability; line count alone is not a reason to rewrite.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Choose validation that matches the change and the repository's requirements. Reuse passing evidence that still covers unchanged files and dependencies; do not add tests that merely repeat wording or implementation details. Examples for behavior changes:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan when it helps coordinate the work:
```text
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Use observable success criteria to complete authorized work independently.
