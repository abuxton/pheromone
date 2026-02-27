# Games Directory

A learning sandbox for **terminal game development** in Go. Each game is self-contained in its own subdirectory with an independent Go module.

## Games

| Game | Description | Controls |
|---|---|---|
| [snake](snake/) | Classic Snake — eat food, grow the snake, don't hit walls or yourself | Arrow keys / WASD |

## Running a Game

```bash
# Run directly
cd games/snake
go run .

# Or build and run
go build -o snake . && ./snake
```

## Requirements

- Go 1.24+ (pre-installed in the devcontainer)
- A terminal that supports Unicode and ANSI colour codes  
  (macOS Terminal ✅, iTerm2 ✅, VS Code integrated terminal ✅, any Linux terminal ✅)

## Learning Resources

See [`../docs/game-dev-resources.md`](../docs/game-dev-resources.md) for a curated list of awesome game-development books, courses, and repositories.

## Adding a New Game

See [`AGENTS.md`](AGENTS.md) for step-by-step instructions and recommended libraries.
