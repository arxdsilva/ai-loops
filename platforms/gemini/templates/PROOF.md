# Proof template

One file per acceptance check, `proofs/AC-<n>[-<case>].md`, written by `product-reviewer` after exercising the real interface (curl, grpcurl, CLI). Trim output to what shows the result.

```markdown
# AC-<n>: <criterion> - case: <happy path | edge case name>

Preconditions: <service version/commit, seed data, config, auth>

Command:
    <exact command>

Output (trimmed):
    <response status/body>

Log excerpt:
    <relevant lines, request id>

Result: PASS|FAIL
Note: <if FAIL: what differs from the AC>
```
