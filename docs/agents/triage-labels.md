# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual tag strings used in this repo's issue tracker (epos).

| Label in mattpocock/skills | Tag in epos tracker  | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned (then close ticket) |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), apply the corresponding tag string via `epos edit <id> --tags "<tag>"`.

Edit the right-hand column to match vocabulary changes.
