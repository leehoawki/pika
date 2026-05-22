package chart

import "fmt"

const (
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	BoldPfx = "\033[1m"
	Reset   = "\033[0m"
)

func Colorize(text, color string) string {
	return fmt.Sprintf("%s%s%s", color, text, Reset)
}

func Bold(text string) string {
	return fmt.Sprintf("%s%s%s", BoldPfx, text, Reset)
}
