# Issues

## Table of Contents

- [How to Open a Feature Request](#how-to-open-a-feature-request)
- [How to Open a Bug Issue](#how-to-open-a-bug-issue)
- [Maintaining an Issue](#maintaining-an-issue)

## How to Open a Feature Request

Opening a feature request before building lets us tell you early if something won't be implemented — saving everyone time.

When opening a new issue on dtwiz, users will be presented with a choice of issue templates. Please fill out the "Feature request" template, describing the problem you're solving and the solution itself. Please also use the `feature` label this request.

Maintainers will review it and compare priority against current work. If there aren't enough resources to pick it up, we'll let you know — and we welcome you to implement it yourself if you're able.

## How to Open a Bug Issue

If you believe that you have uncovered a bug, please fill out the "Bug report" template to the best of your ability and use the `bug` label on the new issue.

The two most important pieces of information we need in order to properly evaluate the report is a description of the behavior you are seeing and a simple test case we can use to recreate the problem on our own. If we cannot recreate the issue, it's not possible to fix it.

Please include the following in your bug report:

1. **Environment** — your operating system and Go version (you can run `go version` in your terminal to find out);
2. **Steps to reproduce**;
3. **Expected behavior** — what should have happened;
4. **Actual behavior** — what happened instead; please include the full error output if there was an error;
5. **Workaround** (_optional_) — if you found one, please share it with us.

## Maintaining an Issue

If you decide to take over the issue, please do the following:

1. Assign yourself to the issue — this helps us quickly see whether the issue needs someone to take it over, as well as provides a reference to the assignee.
2. When you open any Pull Request (PR) which contributes to a certain feature or bugfix, please reference that issue in the `Development` section of the PR page on the right. This helps us quickly understand which issue you're trying to solve.
3. If you have any updates, thoughts, or resources that you consider important to share, please provide those as part of a comment to the issue you're working on, so that we can understand more details about your work.
4. (_Optional_) If you believe your current issue is either blocked by or blocking some other existing issue, please specify that in the `Relationships` section of the issue page on the right.
