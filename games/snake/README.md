# Snake Game

A classic Snake game for the terminal, written in Go using the [tcell](https://github.com/gdamore/tcell) library. Runs on macOS Terminal, iTerm2, Linux terminals, and any Codespace/devcontainer that has a TTY.

## Quick Start

```bash
cd games/snake
go run .
```

Or build once and run:

```bash
cd games/snake
go build -o snake .
./snake
```

## Controls

| Key | Action |
|---|---|
| `↑` / `W` | Move up |
| `↓` / `S` | Move down |
| `←` / `A` | Move left |
| `→` / `D` | Move right |
| `P` | Pause / Resume |
| `R` | Restart (after game over) |
| `Q` / `Esc` | Quit |

## Requirements

- Go 1.24+  (installed automatically in the devcontainer)
- A terminal emulator that supports Unicode and ANSI colours (macOS Terminal, iTerm2, or any modern terminal)

## How to Play

- Guide the snake (►) to eat the food (●).
- The snake grows longer each time it eats.
- The game ends if the snake hits a wall or itself.
- Try to beat your high score!

## Architecture

```
games/snake/
├── go.mod      Isolated Go module (does not affect the root module)
├── go.sum      Dependency hashes
└── main.go     All game logic: state, rendering, input handling
```

The game uses a single `Game` struct to hold all state, a `tcell.Screen` for rendering, and a simple ticker-driven game loop. Input is handled asynchronously via a goroutine that feeds events into a channel.

## Next Steps

See [`docs/game-dev-resources.md`](../../docs/game-dev-resources.md) for curated learning resources, and [`games/AGENTS.md`](../AGENTS.md) for Copilot guidance on extending this game or building new ones.
