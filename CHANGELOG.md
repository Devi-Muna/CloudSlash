# Changelog

## [v1.3.2] - 2026-01-05

### "Future-Glass" TUI & Advanced Forensics

**Feature Highlights:**

- **The "Obsidian" HUD**: Persistent top-bar monitoring Waste ($), Risk, and Scan Status.
- **Deep Drill-Down**: Press `Enter` on any resource to view extended intel (Tags, Cost, Risk Score).
- **"Sparklines"**: Visual ASCII charts (`[__▄_█]`) in the details view showing metric history.
- **"Blame" Attribution**: Instantly identifying resource owners (`IAM:admin` or `Tag:Owner`).
- **"Guilt Trip" Math**: displaying annualized costs (`$384/yr`) in red to drive urgency.

**New Capabilities:**

- **Blast Radius Simulator**: `nuke` now checks for upstream dependencies before deletion.
- **Soft Delete (`m`)**: "Mark" resources for later deletion instead of immediate nuking.
- **Clipboard Ninja**: Copy IDs (`y`), ARNs (`shift+Y`), or JSON (`c`) directly from the terminal.
- **Smart Filters**: Sort by Price (`P`), Filter Easy Wins (`E`), or cycle Regions (`R`).
- **Paper Trail**: All destructive actions are logged to `~/.cloudslash/audit.log`.

**Improvements:**

- **Aesthetic**: Complete removal of emojis in favor of professional text beacons (`[CRITICAL]`, `[SAFE]`).
- **Architecture**: Modular TUI codebase (`model`, `view`, `hud`) using Bubble Tea.
- **Safety**: Enhanced dependency graph resolution.
