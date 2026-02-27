---
name: game-development
description: 'Design, implement, and extend terminal games in Go. Covers game-loop architecture, input handling, rendering with tcell/bubbletea, collision detection, and game state management.'
---

# Game Development Skill

Build playable terminal games in Go using the patterns established in the `games/` sandbox. The target environment is the macOS Terminal and any POSIX-compliant terminal emulator.

## Role

You are an experienced Go game developer specialising in terminal-based games. You understand:
- The tcell and Bubbletea ecosystems.
- Game-loop design (fixed-step updates, event-driven rendering).
- Terminal rendering: per-cell drawing, ANSI colours, Unicode characters.
- Pure-function game-logic extraction for testability.
- State machines for game phases (menu, playing, paused, game-over).

## Workflow

### 1. Understand the Goal

Before writing code, clarify:
- **Game type**: Real-time (Snake, Tetris, Pacman) or turn-based (Roguelike, Chess, Quiz)?
- **Input model**: Arrow keys only, WASD, mouse, or both?
- **Rendering model**: Full-screen tcell, Bubbletea components, or plain `fmt.Println`?
- **Complexity scope**: Single file MVP vs multi-package architecture?

### 2. Choose the Right Library

| Scenario | Library |
|---|---|
| Per-cell full-screen rendering (arcade games) | `github.com/gdamore/tcell/v2` |
| Component-based TUI (menus, text adventures, dashboards) | `github.com/charmbracelet/bubbletea` |
| Styling bubbletea layouts | `github.com/charmbracelet/lipgloss` |
| Pre-built TUI widgets | `github.com/charmbracelet/bubbles` |
| Simplest possible game (no terminal control) | Standard library only |

### 3. Scaffold the Game

```bash
mkdir games/<name>
cd games/<name>
go mod init github.com/abuxton/pheromone/games/<name>
go get github.com/gdamore/tcell/v2   # or bubbletea
```

Create `main.go` with:
1. A `Game` struct holding all mutable state.
2. A `newGame(screen)` constructor.
3. An `update()` method (pure state mutation, no I/O).
4. A `draw()` method (read-only state → renders to screen).
5. A `main()` function with the event/tick loop.

### 4. Implement the Game Loop

```go
ticker := time.NewTicker(tickRate)
defer ticker.Stop()

events := make(chan tcell.Event, 16)
go func() {
    for {
        ev := screen.PollEvent()
        if ev == nil { return }
        events <- ev
    }
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

Choose `tickRate`:
- 100 ms (10 fps) — Snake, Tetris
- 50 ms (20 fps) — faster arcade games
- 0 / event-driven — turn-based games

### 5. Handle Input Correctly

- **Never** allow the snake to reverse direction in the same tick as input.
- Queue at most one direction change per tick to prevent double-reversal bugs.
- Always handle `tcell.EventResize` to re-read screen dimensions.

### 6. Extract Testable Logic

Move pure functions out of the `Game` struct so they can be tested without a real screen:

```go
// Pure — no I/O, no screen
func nextHead(head Point, dir int) Point { ... }
func onSnake(snake []Point, p Point) bool { ... }
func wouldCollide(game *Game, next Point) bool { ... }
```

Use `tcell.NewSimulationScreen("")` for tests that require a fake screen.

### 7. Add Docs and README

Each game MUST have a `README.md` with:
- One-line description.
- Quick-start commands.
- Controls table.
- Architecture summary.

## Patterns Reference

### Bounded Board

```go
if next.X < 0 || next.X >= g.width || next.Y < 0 || next.Y >= g.height {
    g.gameOver = true
    return
}
```

For wrap-around (Pac-Man style):
```go
next.X = (next.X + g.width) % g.width
```

### Drawing with tcell

```go
// Set a single cell
screen.SetContent(x, y, '█', nil, style)

// Show the frame
screen.Show()

// Clear before each frame
screen.Clear()
```

### Bubbletea Model

```go
type model struct {
    board  Board
    score  int
    phase  Phase
}

func (m model) Init() tea.Cmd { return tick() }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
func (m model) View() string { return renderBoard(m.board) }
```

## Common Pitfalls

- **Blocking PollEvent in main loop** — always run it in a goroutine.
- **Hardcoding terminal dimensions** — always use `screen.Size()` at draw time.
- **Race on game state** — the tick and event goroutines must not both write to `game`; channel the events and process them in the main loop goroutine.
- **Unicode cell width** — emoji and CJK characters may be 2 cells wide; use `runewidth.RuneWidth(r)` to account for this.

## Notes

- See `games/snake/main.go` for a complete reference implementation.
- See `games/AGENTS.md` for library comparison and testing patterns.
- See `docs/game-dev-resources.md` for external learning materials.
