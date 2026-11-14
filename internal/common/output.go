package common

import "github.com/fatih/color"

// Progress runes are the frames of the spinner shown while working.
var (
	ProgressRunes      = []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)
	ASCIIProgressRunes = []rune(`-\\|/`)
)

// SpinnerRunes returns the frames of the spinner, in ASCII when noUnicode.
func SpinnerRunes(noUnicode bool) []rune {
	if noUnicode {
		return ASCIIProgressRunes
	}
	return ProgressRunes
}

// BoldColor returns a bold colour of the foreground fg, which prints no
// colour when enabled is false. When it is true, the colour still follows
// the terminal, as NO_COLOR and TERM=dumb tell.
func BoldColor(fg color.Attribute, enabled bool) *color.Color {
	c := color.New(fg).Add(color.Bold)
	if !enabled {
		c.DisableColor()
	}
	return c
}
