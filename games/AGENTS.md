# Games — AGENTS.md

## Purpose

This directory is a learning sandbox for **terminal game development** in Go. It is intentionally separate from the Pheromone platform code so that experiments here do not affect production components.

## Structure

```
games/
├── AGENTS.md          This file
├── README.md          Overview and quick-start
└── snake/             Classic Snake game (tcell, single-file)
    ├── go.mod         Isolated Go module
    ├── go.sum         Dependency lock
    ├── main.go        Game logic, rendering, input
    └── README.md      Game-specific docs
```

## How to Add a New Game

1. Create a subdirectory: `games/<game-name>/`
2. Initialise a new Go module: `go mod init github.com/abuxton/pheromone/games/<game-name>`
3. Add `main.go` with `package main` and a `main()` entry point.
4. Use `tcell/v2` for full-terminal rendering or `bubbletea` for component-based TUI apps.
5. Add a `README.md` documenting controls and architecture.
6. Optionally add a `*_test.go` covering the core game-logic functions (update, collision, scoring).

## Libraries & Their Use Cases

| Library | Best For |
|---|---|
| [`github.com/gdamore/tcell/v2`](https://github.com/gdamore/tcell) | Low-level terminal control: custom rendering, per-cell drawing (Snake, Tetris, Roguelikes) |
| [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) | Elm-architecture TUI apps: menus, forms, chat UIs, text adventures |
| [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) | Styling and layout for bubbletea components |
| [`github.com/charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) | Pre-built TUI components (lists, text inputs, spinners, progress bars) |
| Standard `time` + `os` packages | Simple word games, quiz games, timer-based text UIs |

## Key Patterns

### Game Loop (tcell style)

```go
ticker := time.NewTicker(100 * time.Millisecond)
events := make(chan tcell.Event, 10)
go func() {
    for { events <- screen.PollEvent() }
}()

for {
    select {
    case <-ticker.C:
        game.update()
        game.draw()
    case ev := <-events:
        game.handleInput(ev)
    }
}
```

### State Machine for Game Phases

Use a `phase` field (`PhaseMenu`, `PhasePlaying`, `PhasePaused`, `PhaseGameOver`) to control what `update()` and `draw()` do. This keeps the loop simple and prevents state corruption.

### Collision Detection

Keep the snake as a `[]Point` slice. The head is `snake[0]`. Collision = check if `next` equals any element in `snake[:len(snake)-1]`, or is out of bounds.

### Responsive Layout

Always read `screen.Size()` at draw time (and in `EventResize`) rather than storing width/height at startup. This makes games work correctly when the terminal window is resized.

## Testing Game Logic

Extract pure functions (e.g., `onSnake`, `nextHead`, `calcScore`) so they can be tested without a real screen:

```go
// game_logic_test.go
func TestCollisionWithSelf(t *testing.T) {
    snake := []Point{{5,5},{4,5},{3,5}}
    assert.True(t, onSnake(snake, Point{4,5}))
}
```

Use `tcell.NewSimulationScreen("")` for tests that require a screen.

## Resources

See [`../../docs/game-dev-resources.md`](../docs/game-dev-resources.md) for curated learning materials on terminal and general game development.
