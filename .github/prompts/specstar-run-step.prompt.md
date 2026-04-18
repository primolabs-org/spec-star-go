---
name: specstar-run-step
description: Execute the next SpecStar implementation step or a specified step using workflow state.
argument-hint: feature=<name> [step=<step-id>]
agent: SpecStar Coordinator
---

Execute SpecStar work for feature `${input:featureName:feature-name}`.

Optional target step:
${input:stepId:leave blank to use workflow state}

Work inside:
- `.agent-specstar/features/${input:featureName:feature-name}/`

Required behavior:
1. Read workflow memory first:
   - `feature-state.json`
   - `design.md`
   - `clarifications.md` if present
   - `steps/`
   - `reviews/`
   - `sessions/`
2. If a step id is provided, validate that it exists and use it as the active target.
3. If no step id is provided, determine the correct next action from workflow state and artifacts.
4. Coordinate the engineer and reviewer agents according to the current workflow.
5. Update `feature-state.json` honestly based on the outcome.
6. Create or update session artifacts needed for continuation.
7. If blocked, write clear clarification questions instead of guessing.
8. Do not silently skip failed review findings.
9. Do not mark work complete without a grounded approval outcome.

Output expectations:
- updated workflow state
- implementation/review artifacts for the executed step
- clear blocked, rejected, or approved outcome