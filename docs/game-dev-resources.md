# Game Development Resources

A curated collection of books, courses, articles, repositories, and tools for learning game development — with a focus on terminal/TUI games and Go, but covering broader game-dev fundamentals that apply everywhere.

---

## Terminal & TUI Game Development (Go)

### Libraries

| Library | Description | Link |
|---|---|---|
| **tcell/v2** | Low-level terminal cell library — the foundation of the snake game in this repo | https://github.com/gdamore/tcell |
| **Bubble Tea** | Elm-architecture TUI framework by Charmbracelet — excellent for menus, text adventures, dashboards | https://github.com/charmbracelet/bubbletea |
| **Lip Gloss** | CSS-like styling for terminal layouts | https://github.com/charmbracelet/lipgloss |
| **Bubbles** | Pre-built TUI components: lists, text inputs, spinners, progress bars | https://github.com/charmbracelet/bubbles |
| **gocui** | Minimalist console UI library with overlapping panels | https://github.com/jroimartin/gocui |

### Example Projects (great for reading source code)

| Project | What to Learn |
|---|---|
| [chroma](https://github.com/alecthomas/chroma) | Syntax-highlighted output in a terminal |
| [slides](https://github.com/maaslalani/slides) | Markdown slide deck in the terminal (bubbletea) |
| [freeze](https://github.com/charmbracelet/freeze) | Screenshot terminal output — lipgloss patterns |
| [gopher-and-friends](https://github.com/maaslalani/gambit) | Terminal chess (tcell) |

---

## Game Development Fundamentals

### Free Online Courses

| Resource | Format | Link |
|---|---|---|
| **CS50's Introduction to Game Development** (Harvard) | Video lectures + projects | https://cs50.harvard.edu/games/ |
| **Game Programming Patterns** by Bob Nystrom | Free book online | https://gameprogrammingpatterns.com |
| **Godot Official Tutorials** | Interactive docs | https://docs.godotengine.org/en/stable/getting_started/step_by_step/ |

### Books

| Title | Author | Why Read It |
|---|---|---|
| *Game Programming Patterns* | Bob Nystrom | The definitive guide to patterns (State, Observer, Flyweight, ECS) — language-agnostic, free online |
| *The Art of Game Design: A Book of Lenses* | Jesse Schell | Teaches *why* games feel fun; crucial before building complex games |
| *Rules of Play* | Salen & Zimmerman | Foundational game design theory |
| *Land of Lisp* | Conrad Barski | Build text-adventure and simple games in Lisp — surprisingly fun, great for understanding game loops |
| *Writing NES Games* | Kevin Hanson | Low-level game dev — understanding constraints makes you a better game programmer |

### YouTube Channels

| Channel | Focus |
|---|---|
| [Goodgis](https://www.youtube.com/@goodgis) | Indie game dev vlog, Godot |
| [The Cherno](https://www.youtube.com/@TheCherno) | C++ game engine from scratch — deep fundamentals |
| [GDC (Game Developers Conference)](https://www.youtube.com/@Gdconf) | Professional talks: design, architecture, postmortems |
| [Brackeys](https://www.youtube.com/@Brackeys) | Unity tutorials (archived but timeless for game-dev concepts) |
| [Sebastiaan Lague](https://www.youtube.com/@SebastianLague) | Coding adventures: procedural generation, physics sims |

---

## Roguelike Development (closest genre to terminal games)

Roguelikes are the quintessential terminal game genre — ASCII art, procedural generation, turn-based logic.

| Resource | Link |
|---|---|
| **Roguelike Tutorial — Python + libtcod** | https://rogueliketutorials.com |
| **r/roguelikedev** community & tutorials | https://www.reddit.com/r/roguelikedev/ |
| **Rust Roguelike Tutorial** (bracket-lib) | https://bfnightly.bracketproductions.com |
| **7-Day Roguelike Challenge** | https://7drl.com |
| **Angband** source (classic Go-friendly patterns) | https://github.com/angband/angband |

---

## Go-Specific Game Dev

| Resource | Description |
|---|---|
| [Ebiten](https://ebitengine.org) | 2D game engine in Go — beyond terminal, but great for learning game loops in Go |
| [Ebiten examples](https://github.com/hajimehoshi/ebiten/tree/main/examples) | Dozens of playable mini-games (flappy bird, minesweeper, blocks) |
| [donburi](https://github.com/yohamta/donburi) | Entity Component System (ECS) for Go |
| [g3n](https://g3n.rocks) | 3D game engine in Go |

---

## Game Design Fundamentals

| Resource | Description |
|---|---|
| [Extra Credits (YouTube)](https://www.youtube.com/@ExtraCredits) | Short video essays on game design concepts (pacing, narrative, mechanics) |
| [Game Maker's Toolkit (YouTube)](https://www.youtube.com/@GMTK) | Deep analysis of game design decisions in popular games |
| [Itch.io](https://itch.io) | Host and discover indie games; participate in game jams |
| [Ludum Dare](https://ldjam.com) | 48/72-hour game jams — best way to ship your first game fast |

---

## Algorithms Useful in Game Development

| Algorithm | Use Case | Reference |
|---|---|---|
| A* Pathfinding | NPC navigation, roguelike enemies | https://www.redblobgames.com/pathfinding/a-star/introduction.html |
| Flood Fill | Area detection, paint-bucket, map generation | https://en.wikipedia.org/wiki/Flood_fill |
| BSP Trees | Procedural dungeon generation | https://www.roguebasin.com/index.php/Basic_BSP_Dungeon_generation |
| Noise (Perlin/Simplex) | Terrain, texture, procedural level generation | https://rtouti.github.io/graphics/perlin-noise-algorithm |
| Cellular Automata | Cave generation, life simulation | https://roguebasin.com/index.php/Cellular_Automata_Method_for_Generating_Random_Cave-Like_Levels |

---

## Awesome Lists

| List | Link |
|---|---|
| awesome-gamedev | https://github.com/Calinou/awesome-gamedev |
| awesome-go (game section) | https://github.com/avelino/awesome-go#game-development |
| awesome-roguelikes | https://github.com/marukrap/awesome-roguelikes |
| awesome-love2d | https://github.com/love2d-community/awesome-love2d |

---

## Suggested Learning Path

1. **Play** the `games/snake/` game in this repo. Read `main.go` — understand the game loop, state struct, and input handling.
2. **Extend** Snake: add a high-score display, speed increases, or wrap-around walls.
3. **Read** *Game Programming Patterns* (free online) — focus on: State, Observer, Update Method, and Game Loop chapters.
4. **Watch** CS50's Introduction to Game Development (Lecture 0: Pong/Snake from scratch).
5. **Build** your own terminal game using the scaffold in `games/AGENTS.md`.
6. **Join** a game jam (Ludum Dare or 7DRL) to ship something real under time pressure.
7. **Explore** Ebiten for 2D graphical games in Go once comfortable with game-loop fundamentals.
