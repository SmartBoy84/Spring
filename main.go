package main

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/gonutz/w32/v2"
)

var (
	fps = 100

	// acceleration magnitudes
	xA = 0
	yA = 1.5 // gravity?

	// dampening
	ySides = 0.9
	xSides = 0.95

	// square size (px) - randomize?
	sizeX = 300
	sizeY = 300
)

var screenX, screenY, refreshInterval int

type Coord struct {
	x, y float64
}

type RawWindow struct {
	win w32.HWND
}

type Window struct {
	rawWindow RawWindow

	initialPos, acceleration, velocity Coord
}

type Windows struct {
	list []*Window
	mu   sync.Mutex

	lastWindow    RawWindow
	lastWindowPos Coord

	held     bool
	heldTime time.Time
}

func init() {
	screenDimensions := *w32.GetWindowRect(w32.HWND(w32.GetDesktopWindow()))
	screenX = int(screenDimensions.Width())
	screenY = int(screenDimensions.Height())
	refreshInterval = 1000 / fps
}

func (window RawWindow) getPos() Coord {
	rect := w32.GetWindowRect(window.win)
	return Coord{x: float64(rect.Left), y: float64(rect.Top)}
}

func (window RawWindow) moveWindow(coords Coord) {
	curDimension := w32.GetWindowRect(window.win)
	w32.MoveWindow(window.win, int(coords.x), int(coords.y), int(curDimension.Width()), int(curDimension.Height()), true)
}

func (window RawWindow) resizeWindow(width, height int) {
	curDimension := w32.GetWindowRect(window.win)
	w32.MoveWindow(window.win, int(curDimension.Left), int(curDimension.Top), width, height, true)
}

func (window RawWindow) getID() w32.DWORD {
	if w32.GetWindowTextLength(window.win) > 0 {
		_, id := w32.GetWindowThreadProcessId(window.win) // windows is weird, goes through a bunch of other windows before reaching the target window
		return id
	} else {
		return 0
	}
}

func getForeground() RawWindow {
	return RawWindow{win: w32.GetForegroundWindow()}
}

func getMouse() Coord {
	x, y, ok := w32.GetCursorPos()
	if ok {
		return Coord{x: float64(x), y: float64(y)}
	} else {
		return Coord{}
	}
}

// explosion key - give random velocities to all windows on screen

// func (windows Windows) revertBack() {
// 	for window := range windows.list {
// 		window.
// 	}
// }

func (windows *Windows) Animate() {
	for {
		for _, window := range windows.list {

			window.rawWindow.resizeWindow(sizeX, sizeY)

			curPos := window.rawWindow.getPos()

			curPos.x += window.velocity.x
			curPos.y += window.velocity.y
			// fmt.Println(curPos, window.velocity.y, window.acceleration.y, window.velocity.y+window.acceleration.y)

			window.rawWindow.moveWindow(curPos)

			if window.velocity.y != 0 {

				if curPos.y >= float64(screenY-sizeY) {
					curPos.y = float64(screenY - sizeY)
					if math.Abs(window.velocity.y) <= window.acceleration.y {
						window.velocity.y = 0 // otherwise it switches signs then gravity pushes it down harder than it pushed up resulting in an infinite jittering effect
					}
				}

				if curPos.y <= 0 { // we need to be extra strict about these two cases because otherwise acceleration can act more in one bounce than another
					curPos.y = 0
				}

				if curPos.y == float64(screenY-sizeY) || curPos.y == 0 {
					window.velocity.y *= -ySides
					window.velocity.x *= xSides
					// window.velocity.y = math.Floor(window.velocity.y)
				}
			}

			if curPos.y < float64(screenY-sizeY) {
				window.velocity.y += window.acceleration.y
			}

			window.velocity.x += window.acceleration.x

			if curPos.x >= float64(screenX-sizeX) || curPos.x <= 0 {
				window.velocity.x *= -1 // don't cause dampening on bounce with side
			}
			// if impacting a boundary and velocity is not in other direction then shrink window?
		}
		time.Sleep(time.Millisecond * time.Duration(refreshInterval))
	}
}

func (windows *Windows) MonitorChange() {
	for {
		if uint8(w32.GetKeyState(1)>>8) == 0 {
			if windows.held {

				windows.held = false

				curWinPos := windows.lastWindow.getPos()
				// frameCount := float64(time.Now().Sub(windows.heldTime).Milliseconds()) / float64(refreshInterval)

				var windowVelocity Coord

				// if frameCount > 0 {

				// I'm not doing distance over time because update the last pos every frame to ensure we get the speed of launch rather than drag
				windowVelocity.x = (curWinPos.x - windows.lastWindowPos.x) // FIX ME
				windowVelocity.y = (curWinPos.y - windows.lastWindowPos.y)
				// }

				if windowVelocity.x != 0 || windowVelocity.y != 0 {
					log.Printf("Adding! Velocity is (%v, %v) pixels/frame\n", windowVelocity.x, windowVelocity.y)

					windows.mu.Lock()
					windows.list = append(windows.list, &Window{
						rawWindow:    windows.lastWindow,
						initialPos:   windows.lastWindowPos,
						acceleration: Coord{x: float64(xA), y: float64(yA)},
						velocity:     windowVelocity,
					})
					windows.mu.Unlock()
				}

				windows.lastWindowPos = Coord{}
				windows.lastWindow = RawWindow{}
				windows.heldTime = time.Time{}
			}
			continue
		}
		window := getForeground()

		for n, k := range windows.list {
			if k.rawWindow.getID() == window.getID() {
				windows.mu.Lock()
				windows.list[n] = windows.list[len(windows.list)-1]
				windows.list = windows.list[:len(windows.list)]
			}
		}

		if window.getID() > 0 && window.getID() != windows.lastWindow.getID() {
			log.Print("Foreground changed!")

			window.resizeWindow(sizeX, sizeY)

			windows.lastWindow = window
			windows.held = true
		}

		if windows.held && float64(time.Now().Sub(windows.heldTime).Milliseconds())/float64(refreshInterval) >= 8 {
			windows.heldTime = time.Now()
			windows.lastWindowPos = windows.lastWindow.getPos()
		}
	}
}

func main() {

	var windows Windows

	go windows.MonitorChange()
	windows.Animate()
	/*
	   go routine constantly monitor if the current active window has been changed
	   If it has then determine if the user is clicking on it
	   If the user clicks on it then resize it to appropriate size and then time the point at which they no longer click it and then use that to determine horizontal and vertical velocity
	   If both are 0 then the window remains suspended otherwise store its position at that point and then gravity takes place
	   Once both velocity vectors have been drained, window returns to where it was
	   Impact with boundaries should compress it depending on magnitude of velocity
	*/

}
