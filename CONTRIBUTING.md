# Contributing to goat-client

Practical how-to for contributors. The *why* behind these rules lives in
goat-trunk's
[`docs/adr/0109-git-collaboration-trunk-based.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0109-git-collaboration-trunk-based.md);
this file is the day-to-day reference for goat-client.

> **Repo-pair reminder.** goat-client (this repo) is the implementation;
> goat-trunk (`dlf-dds/DesertBreadBird`) is the design. ADRs, design
> docs, and the implementation plan live in goat-trunk and are not
> mirrored here. Cross-link by URL when a goat-client PR is driven by a
> goat-trunk decision.

> **This is a starting point, not a stone tablet.** While the team and
> the practice find their level, the maintainer (`@dlf-dds`) personally
> manages commits and PRs and grants exceptions where the rule doesn't
> yet fit reality. If something here gets in your way, say so in the PR
> — disagreement is feedback. Expect this file (and the linked ADR, and
> CODEOWNERS, and the workflows) to be edited as we learn.

---

## First-time setup

```bash
git clone ssh://git@gh-dlfdds/dlf-dds/goat-client.git
cd goat-client

# Sign your commits with SSH (preferred — same key you use for git push)
git config --global gpg.format ssh
git config --global user.signingkey ~/.ssh/id_ed25519.pub
git config --global commit.gpgsign true
git config --global tag.gpgsign true

# Register the public half with GitHub as a *signing* key (not just an auth key):
#   GitHub → Settings → SSH and GPG keys → New SSH key → Key type: Signing Key

# DCO sign-off on every commit
git config --global format.signoff true

# Verify
git commit --allow-empty -m "test: signing setup" -s
git log --show-signature -1
# Look for: "Good "ssh" signature" and a "Signed-off-by:" trailer
```

GPG-format signing also works — see GitHub's
[commit-signature-verification docs](https://docs.github.com/en/authentication/managing-commit-signature-verification).
The policy is signed-commits-required, not key-format-required.

---

## Branching

- One long-lived branch: `main`. Direct push is blocked.
- Feature branches are short-lived (target: < 5 working days from
  branch to merge).
- **Naming: `track/<short-name>`** for the parallel-track workstreams
  enumerated in [`HANDOFF.md`](HANDOFF.md) — `track/desktop-spine`,
  `track/fyne-ui`, `track/ci-matrix`, `track/governance-docs`, etc.
  For ad-hoc work outside a HANDOFF track, use the conventional-type
  prefix instead (`fix/<slug>`, `docs/<slug>`, `chore/<slug>`).
- No `develop`, no permanent release branches. Hotfixes branch from
  `main` and merge back to `main`.

```bash
cd /Users/dene/src/github.com/dlf-dds/goat-client/
git fetch origin && git checkout main && git pull --ff-only

# For a HANDOFF track — use the per-session worktree convention:
/iso enter <track-name>   # creates .claude/worktrees/<track-name>/ off origin/main

# For ad-hoc work:
git checkout -b fix/<slug>
```

---

## Commits

**Sign every commit.** Branch protection on `main` rejects unsigned
commits.

**DCO sign-off on every commit.** `git commit -s` (or `format.signoff =
true` from setup above). The trailer is `Signed-off-by: Name <email>`;
it must match the commit author.

**Track trailer.** Commits that belong to a HANDOFF track include a
`[track: <name>]` trailer in the commit body. Multiple commits on the
same branch share the trailer; the squashed PR commit on `main`
inherits it. Example:

```text
ui: wire bundle-import dialog to IPC stub

Renders bundle metadata (issued-to / site / expires) from the parsed
CBOR before Apply is enabled.

[track: goat-client-fyne-ui]
Signed-off-by: Dene Farrell <dene.farrell@gmail.com>
```

**No `Co-Authored-By:` trailers added by AI tooling.** Pair-programmed
work is fine; the trailer must be authored by a human contributor.

**Conventional Commits for PR titles.** The PR title becomes the
squash-merge commit message on `main`, so it must parse cleanly:

```text
<type>(<scope>): <subject>

# Examples
feat(tunnel): single-peer wg-cp0 iface manager
fix(ipc): handle truncated bundle CBOR gracefully
ui(fyne): bundle-import dialog with drag-drop + file-picker
ci: six-target release matrix with cosign signing
docs(handoff): mark Track B ready-for-review
chore(deps): bump fyne to v2.5.x
```

Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`,
`perf`, `ci`, `build`, `revert`, `ui`. PR-title check enforces this
once it lands in CI.

**Commit body** (optional but encouraged): the *why* the diff doesn't
already make obvious. Reference the goat-trunk design doc / ADR section
or the HANDOFF track if relevant.

---

## Pull requests

Open with the GitHub CLI:

```bash
gh pr create --fill
# or for a draft while still iterating:
gh pr create --draft --fill
```

PR body should cover:

- **What changed and why** (1–3 sentences). Cite the HANDOFF track, the
  goat-trunk design doc section, or the netbird path being forked.
- **Test plan** — what you verified, how a reviewer can re-verify.
- **Acceptance criterion** — the bullet from HANDOFF.md this PR closes
  (or which subset, if the track lands in pieces).
- **Cross-repo coordination**, if any — link sibling PRs in goat-trunk
  (e.g., a design-doc update that lands together).

**Merge gates** (all required, all enforced by branch protection):

- All required CI checks green (vet / test / build, cross-compile sanity,
  actionlint — see Track E's workflow scaffold).
- Signed-commits check green.
- DCO check green.
- PR-title is valid Conventional Commits.
- ≥ 1 approving review (≥ 2 for security-sensitive paths once
  `CODEOWNERS` lands them — crypto, bundle parser, IPC auth boundary).
- All conversations resolved.
- Branch up-to-date with `main`.

**Merge style**: **squash-and-merge**. The PR title becomes the single
signed commit on `main`. Rebase-merge is allowed for stacked PRs where
preserving the chain has review value. No merge commits on `main`.

**Don't bypass.** No `--no-verify`, no admin-override on branch
protection. If a required check is flaky, fix the test or take it out
of the required list — never skip the gate.

---

## CODEOWNERS

[`CODEOWNERS`](CODEOWNERS) at the repo root maps directories to owning
teams. PRs touching an owned path auto-request review. Security-
sensitive paths require **two** approvals — listed in the file's
header comment, not memorized. Keep it current; stale owners block
merges. If you're adding a new top-level directory, add an owner line
in the same PR.

---

## Cross-repo coordination with goat-trunk

A change that touches both repos picks one of these patterns at PR-open
time and puts it in the PR body:

1. **Sequenced merge** *(default)*. Land in dependency order: goat-trunk
   ADR / design-doc update merges first; the goat-client PR cites the
   goat-trunk PR by URL and merges after. Cross-link in both bodies.
2. **Atomic via design parity**. The goat-client implementation merges
   alongside a goat-trunk doc update authored in the same session; both
   PRs reference each other and reviewers read both.
3. **Implementation-only**. No goat-trunk change required (most tracks).
   The PR cites the existing goat-trunk design doc by URL.

Default is (1) when an ADR clarification or design-doc revision is
prompted by the implementation work. (3) is the steady state for
within-design-envelope work.

---

## When CI fails

1. Read the failing job's log. Don't retry without reading.
2. If it's your code: fix locally, re-push to the same branch.
3. If it's a flaky test: open an issue, link from your PR, and either
   fix the flake or take the check out of the required list — *never*
   merge with the gate disabled.
4. If a hook fails on commit: investigate, don't `--no-verify`.

---

## Worktree discipline

Per HANDOFF.md, parallel-track work uses per-session worktrees
provisioned under `.claude/worktrees/<track-name>/`. Two ground rules:

- **Master checkout is read-only.** All `Edit`/`Write` target the
  worktree path. Same file-level invariant as goat-trunk's ADR 0013
  Amendment 2026-05-09.
- **One track per worktree.** Don't sweep files from another track into
  your branch. If you find yourself touching files that belong to
  someone else's track, stop and coordinate.

---

## Releases

Tags are cut from `main` only, on a green build. Tag format:
`goat-client-v<MAJOR>.<MINOR>.<PATCH>` (the `goat-client-` prefix
namespaces this repo's tags from any future siblings). Track E's release
workflow produces six desktop binaries + per-asset `.sha256` +
aggregate `SHA256SUMS` + cosign signatures attached to the GitHub
Release on tag push.

---

## Where to ask

- Implementation question for a track → comment on the relevant PR or
  open a draft PR with the question in the body.
- Architectural / decision question → open a draft ADR in goat-trunk;
  link it from the goat-client PR that raised the question.
- Security concern → see [`SECURITY.md`](SECURITY.md). Do not open a
  public issue for vulnerabilities.
