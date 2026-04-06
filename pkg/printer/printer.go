package printer

import "fmt"

type Color string

const (
	ColorReset  Color = "\033[0m"
	ColorRed    Color = "\033[31m"
	ColorGreen  Color = "\033[32m"
	ColorYellow Color = "\033[33m"
	ColorBlue   Color = "\033[34m"
	ColorCyan   Color = "\033[36m"
)

func Print(msg string, arg ...any) {
	fmt.Printf(msg, arg...)
}

func SprintColor(color Color, msg string, arg ...any) string {
	return fmt.Sprintf("%s%s%s", color, fmt.Sprintf(msg, arg...), ColorReset)
}
