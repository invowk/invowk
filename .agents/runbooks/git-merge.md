# Squash Merge

Merging into `main` is squash-only by project policy (`.agents/rules/git.md`). The repository settings still enable merge-commit and rebase merging (the "Safety" ruleset requires signatures, linear history and the `goplint consumer gates` check, not squash), so pick squash explicitly in the UI, or merge locally:

```bash
git checkout main
git merge --squash feature-branch
git commit -S -F - <<'EOF'
feat(scope): summary

- Bullet points describing changes

Co-authored-by: Name <email>
EOF
```

After the merge, delete the local branch (`git branch -d feature-branch`) and the remote one (`git push origin --delete feature-branch`). Delete only branches the current session created.

Why squash: each feature lands as one signed, atomic commit with a cohesive message, which keeps `main` linear and makes reverts easy.
