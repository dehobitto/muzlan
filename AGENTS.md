# Project Instructions

We are building this project with a trunk-based development approach: small, short-lived branches with reviewable pull requests back into `main`.

## Engineering Principles

- Write testable code and add tests where behavior needs protection.
- Keep changes focused and easy to review.
- Favor clear behavior over clever implementation.
- When a feature or task is unclear, call out the uncertainty before building.
- If the proposed approach seems wrong or risky, say so directly and explain the concern.

## Workflow

- Create a new branch for each feature or meaningful change.
- Open a pull request from that feature branch into `main`.
- Do not merge directly into `main`.
- The user will review pull requests and decide what to merge or change.

## Review Expectations

- Explain what changed in each pull request.
- Include relevant tests or explain why tests were not added.
- Keep pull requests small enough that they can be reviewed comfortably.
