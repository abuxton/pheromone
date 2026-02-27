package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Direction constants.
const (
	DirUp = iota
	DirDown
	DirLeft
	DirRight
)

// Point represents a 2D coordinate on the board.
type Point struct {
	X, Y int
}

// Game holds all mutable game state.
type Game struct {
	screen    tcell.Screen
	snake     []Point
	dir       int
	food      Point
	score     int
	width     int
	height    int
	gameOver  bool
	paused    bool
}

// styles used throughout the game.
var (
	styleSnakeHead = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	styleSnakeBody = tcell.StyleDefault.Foreground(tcell.ColorDarkGreen)
	styleFood      = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleBorder    = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	styleScore     = tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	styleGameOver  = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleInfo      = tcell.StyleDefault.Foreground(tcell.ColorSilver)
)

func newGame(s tcell.Screen) *Game {
	w, h := s.Size()
	// Board is inset by 1 for the border; header takes 2 rows at the top.
	boardW := w - 2
	boardH := h - 4

	startX := boardW / 2
	startY := boardH / 2

	g := &Game{
		screen: s,
		dir:    DirRight,
		width:  boardW,
		height: boardH,
		snake: []Point{
			{startX, startY},
			{startX - 1, startY},
			{startX - 2, startY},
		},
	}
	g.placeFood()
	return g
}

// placeFood puts food at a random empty cell.
func (g *Game) placeFood() {
	for {
		p := Point{rand.Intn(g.width), rand.Intn(g.height)}
		if !g.onSnake(p) {
			g.food = p
			return
		}
	}
}

func (g *Game) onSnake(p Point) bool {
	for _, s := range g.snake {
		if s == p {
			return true
		}
	}
	return false
}

// update advances the game by one tick.
func (g *Game) update() {
	if g.gameOver || g.paused {
		return
	}

	head := g.snake[0]
	var next Point
	switch g.dir {
	case DirUp:
		next = Point{head.X, head.Y - 1}
	case DirDown:
		next = Point{head.X, head.Y + 1}
	case DirLeft:
		next = Point{head.X - 1, head.Y}
	case DirRight:
		next = Point{head.X + 1, head.Y}
	}

	// Wall collision.
	if next.X < 0 || next.X >= g.width || next.Y < 0 || next.Y >= g.height {
		g.gameOver = true
		return
	}

	// Self collision.
	if g.onSnake(next) {
		g.gameOver = true
		return
	}

	// Prepend the new head.
	g.snake = append([]Point{next}, g.snake...)

	if next == g.food {
		g.score++
		g.placeFood()
	} else {
		// Remove the tail unless we just ate.
		g.snake = g.snake[:len(g.snake)-1]
	}
}

// draw renders the full game frame.
func (g *Game) draw() {
	g.screen.Clear()
	w, _ := g.screen.Size()

	// Header.
	title := "🐍  SNAKE  —  use arrow keys or WASD  |  P = pause  |  Q = quit"
	drawText(g.screen, 0, 0, styleInfo, title)
	scoreStr := fmt.Sprintf("Score: %d   Length: %d", g.score, len(g.snake))
	drawText(g.screen, 0, 1, styleScore, scoreStr)

	// Border (offset by 2 rows for header).
	offsetY := 2
	for x := 0; x <= g.width+1; x++ {
		g.screen.SetContent(x, offsetY, '─', nil, styleBorder)
		g.screen.SetContent(x, offsetY+g.height+1, '─', nil, styleBorder)
	}
	for y := offsetY; y <= offsetY+g.height+1; y++ {
		g.screen.SetContent(0, y, '│', nil, styleBorder)
		g.screen.SetContent(g.width+1, y, '│', nil, styleBorder)
	}
	g.screen.SetContent(0, offsetY, '┌', nil, styleBorder)
	g.screen.SetContent(g.width+1, offsetY, '┐', nil, styleBorder)
	g.screen.SetContent(0, offsetY+g.height+1, '└', nil, styleBorder)
	g.screen.SetContent(g.width+1, offsetY+g.height+1, '┘', nil, styleBorder)

	// Food.
	g.screen.SetContent(g.food.X+1, g.food.Y+offsetY+1, '●', nil, styleFood)

	// Snake.
	for i, p := range g.snake {
		ch := '█'
		st := styleSnakeBody
		if i == 0 {
			st = styleSnakeHead
			// Head direction indicator.
			switch g.dir {
			case DirUp:
				ch = '▲'
			case DirDown:
				ch = '▼'
			case DirLeft:
				ch = '◄'
			case DirRight:
				ch = '►'
			}
		}
		g.screen.SetContent(p.X+1, p.Y+offsetY+1, ch, nil, st)
	}

	// Overlay messages.
	if g.paused {
		msg := "  PAUSED — press P to resume  "
		drawCentered(g.screen, w, offsetY+g.height/2, styleGameOver, msg)
	}
	if g.gameOver {
		msg := fmt.Sprintf("  GAME OVER!  Score: %d  — press R to restart or Q to quit  ", g.score)
		drawCentered(g.screen, w, offsetY+g.height/2, styleGameOver, msg)
	}

	g.screen.Show()
}

func drawText(s tcell.Screen, x, y int, st tcell.Style, text string) {
	for i, r := range []rune(text) {
		s.SetContent(x+i, y, r, nil, st)
	}
}

func drawCentered(s tcell.Screen, w, y int, st tcell.Style, text string) {
	runes := []rune(text)
	x := (w - len(runes)) / 2
	if x < 0 {
		x = 0
	}
	drawText(s, x, y, st, text)
}

func main() {
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create screen: %v\n", err)
		os.Exit(1)
	}
	if err = s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to init screen: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()

	s.EnableMouse()
	s.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack))

	game := newGame(s)

	// Tick channel drives game speed (10 frames/sec = 100 ms).
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Event channel.
	events := make(chan tcell.Event, 10)
	go func() {
		for {
			ev := s.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()

	game.draw()

	for {
		select {
		case <-ticker.C:
			game.update()
			game.draw()

		case ev := <-events:
			switch e := ev.(type) {
			case *tcell.EventKey:
				switch {
				case e.Key() == tcell.KeyEscape || e.Rune() == 'q' || e.Rune() == 'Q':
					return
				case e.Rune() == 'p' || e.Rune() == 'P':
					if !game.gameOver {
						game.paused = !game.paused
					}
				case e.Rune() == 'r' || e.Rune() == 'R':
					if game.gameOver {
						game = newGame(s)
					}
				case e.Key() == tcell.KeyUp || e.Rune() == 'w' || e.Rune() == 'W':
					if game.dir != DirDown {
						game.dir = DirUp
					}
				case e.Key() == tcell.KeyDown || e.Rune() == 's' || e.Rune() == 'S':
					if game.dir != DirUp {
						game.dir = DirDown
					}
				case e.Key() == tcell.KeyLeft || e.Rune() == 'a' || e.Rune() == 'A':
					if game.dir != DirRight {
						game.dir = DirLeft
					}
				case e.Key() == tcell.KeyRight || e.Rune() == 'd' || e.Rune() == 'D':
					if game.dir != DirLeft {
						game.dir = DirRight
					}
				}

			case *tcell.EventResize:
				s.Sync()
				// Rebuild board geometry on resize.
				w, h := s.Size()
				game.width = w - 2
				game.height = h - 4
				game.draw()
			}
		}
	}
}
