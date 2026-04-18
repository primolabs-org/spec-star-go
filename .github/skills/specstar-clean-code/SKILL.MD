---
name: "specstar-clean-code"
description: "A skill for writing clean, maintainable code that follows best practices and design principles."
user-invocable: false
---

# SpecStar Clean Code

Apply this skill to implementation, refactoring and review work.

## Authority order

Always apply rules in this order:

1. Active SpecStar step file or fix-step
2. Feature design.md
3. Repository instructions and scaffolding rules
4. This skill

Do not override higher-priority constraints.

## Purpose

This skill enforces implementation-level clean code defaults.

It is intended to reduce:
- cognitive complexity
- avoidable duplication
- dead code
- unused parameters
- oversized functions
- unclear control flow
- weak naming
- comments without value

This skill does not redesign architecture, it improves code quality within the allowed scope.

## Core defaults

Use these defaults unless a higher-priority instruction says otherwise: 

- cognitive complexity per function should be <= 15
- avoid deeply nested control flow
- prefer small, focused functions
- remove unused parameters unless required by an interface
- remove dead local variables
- remove unused imports
- determine if a function output is used, if not, discard it with a `_` assignment if allowed by the language
- avoid duplicated literals when extraction improves clarity
- prefer explicit names over short or ambiguous names
- prefer straightforward branching over clever compactness

## Function design rules

Prefer functions that:

- do one thing
- have one clear level of abstraction
- are easy to scan top-to-bottom
-can be understood without reading large amounts of surrounding code

Avoid functions that:

- violates the single responsibility principle
- mix multiple levels of abstraction
- contain many nested branches
- carry unused arguments
- hide important behavior in compact expressions

## Cognitive complexity rules

Default target:
- keep cognitive complexity per function <= 15

When a function approaches or exceeds that threshold, prefer:
- extracting named helper functions
- using guard clauses
- flattening nested conditionals
- splitting independent concerns
- replacing long conditional chains with clearer dispatch or helper logic when appropriate

Do not split functions mechanically if the result becomes harder to understand.

## Duplication rules

Reduce duplication when it is meaningful and local.

Prefer extraction when:
- the same literal appears multiple times with the same meaning
- the same branching logic appears in multiple places
- the same formatting or validation rule is repeated
- repeated code expresses one stable concept

Do not extract:
- trivial one-off values
- constants that become less clear after extraction
- abstractions with no stable meaning

## Literal handling rules

Avoid repeated magic literals when they represent stable business or technical meaning.

Examples:
- repeated status values
- repeated component names
- repeated file suffixes
- repeated error reasons
- repeated field names used as true constants

Prefer:
- module-level named constants
- clearly named local constants when scope is narrow

Do not replace every repeated string with a constant blindly.
Only extract when it improves readability, consistency, or maintainability.

## Parameter rules

Remove unused parameters unless:
- the parameter is required by an external interface
- the parameter is part of an abstract or framework contract

## Variable and naming rules

Prefer names that make the code self-explanatory.

Good names should communicate:
- what the value represents
- whether it is raw, validated, formatted or final
- whether it is a count, identifier, flag or collection

Avoid:
- vague names like `data`, `result`, `value`, `obj` when more specific naming is possible
- misleading abbreviations
- names reused across different abstraction levels in the same function

## Control flow rules

Prefer:
- guard clauses
- early exits
- clear branch separation
- one obvious path for normal flow

Avoid:
- nested `if`/`else` pyramids
- long chains of mixed branching and mutation
- broad exception handling unless it is the top-level orchestrator or a tightly scoped translation point
- boolean flags that control too many downstream decisions

## Comment rules

Prefer code that explains itself throught structure and naming.

Use comments only when needed to clarify:
- non-obvious intent
- external constraints
- protocol/layout requirements
- justified tradeoffs

Avoid comments that:
- restate the code
- are stale after refactoring

## Dead code rules

Remove:
- unused parameters
- unused locals
- unreachable branches
- leftover helper functions
- stale compatibility code no longer needed

Do not leave placeholders, speculative helpers or compatibility code unless explicitly required.

## Error-prone patterns to avoid

- long procedural functions
- mixed abstraction levels in one function
- repeated magic strings
- partial refactors that leave unused artifacts
- helper functions used only to hide complexity without improving clarity
- broad utility modules with unrelated helpers

## Review guidance

When reviewing code, check for:
- functions with excessive branching or nesting
- duplicated meaningful literals
- unused parameters or variables
- mixed responsabilities in one function
- confusing names
- stale comments
- hidden dead code after refactors
- helpers that add indirection but not clarity

## Forbidden behaviors

- introducing abstractions only to satisfy a metric
- splitting code into too many tiny functions with weak names
- extracting constants for trivial one-off values
- preserving unused parameters without justification
- leaving dead code after refactor
- hiding complexity behind vague helper names
