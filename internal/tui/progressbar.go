package tui

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProgressBar is a tview primitive that renders a horizontal progress bar.
// It embeds tview.Box so it participates in layout and can optionally have
// a border and title.
type ProgressBar struct {
	*tview.Box

	progress int // 0–100
	useColor bool

	mu sync.RWMutex
}

// NewProgressBar returns a new ProgressBar with default settings.
func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		Box: tview.NewBox(),
	}
}

// SetUseColor controls whether the filled segment is highlighted with colour.
func (p *ProgressBar) SetUseColor(use bool) *ProgressBar {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.useColor = use
	return p
}

// SetProgress sets the current progress in the range [0, 100].
func (p *ProgressBar) SetProgress(progress int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}
	p.progress = progress
}

// GetProgress returns the current progress in the range [0, 100].
func (p *ProgressBar) GetProgress() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.progress
}

// Draw implements tview.Primitive. It draws the border (via Box.Draw) and
// then fills the inner rect with a segmented progress bar.
func (p *ProgressBar) Draw(screen tcell.Screen) {
	p.Box.Draw(screen)

	p.mu.RLock()
	progress := p.progress
	useColor := p.useColor
	p.mu.RUnlock()

	x, y, width, height := p.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	percentStr := fmt.Sprintf(" %3d%% ", progress)
	barWidth := width - len(percentStr)
	if barWidth < 0 {
		barWidth = 0
	}

	filled := 0
	if barWidth > 0 && progress > 0 {
		filled = barWidth * progress / 100
	}

	filledStyle := tcell.StyleDefault.Foreground(tcell.ColorDefault)
	if useColor {
		filledStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	}
	emptyStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	textStyle := tcell.StyleDefault.Foreground(tcell.ColorDefault)

	// Render only the middle row; if height > 1 the box padding handles spacing.
	row := (height - 1) / 2
	for col := 0; col < barWidth; col++ {
		ch := '░'
		style := emptyStyle
		if col < filled {
			ch = '█'
			style = filledStyle
		}
		screen.SetContent(x+col, y+row, ch, nil, style)
	}
	for i, ch := range []rune(percentStr) {
		if barWidth+i >= width {
			break
		}
		screen.SetContent(x+barWidth+i, y+row, ch, nil, textStyle)
	}
}
