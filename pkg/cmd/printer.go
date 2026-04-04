package cmd

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

type Color string

const (
	ColorReset  Color = "\033[0m"
	ColorRed    Color = "\033[31m"
	ColorGreen  Color = "\033[32m"
	ColorYellow Color = "\033[33m"
	ColorBlue   Color = "\033[34m"
	ColorCyan   Color = "\033[36m"
)

func Colorlist() []Color {
	return []Color{ColorRed, ColorGreen, ColorYellow, ColorBlue, ColorCyan}
}

func Print(msg string, arg ...any) {
	fmt.Printf(msg, arg...)
}

func PrintEmptyLine() {
	fmt.Println()
}

func printStatus(color Color, label string, msg string, arg ...any) {
	fmt.Printf("%s[%s]%s %s", color, label, ColorReset, fmt.Sprintf(msg, arg...))
}

func PrintInfo(msg string, arg ...any) {
	printStatus(ColorBlue, "*", msg, arg...)
}

func PrintOK(msg string, arg ...any) {
	printStatus(ColorGreen, "OK", msg, arg...)
}

func PrintWarn(msg string, arg ...any) {
	printStatus(ColorYellow, "WARN", msg, arg...)
}

func PrintErr(msg string, arg ...any) {
	printStatus(ColorRed, "ERR", msg, arg...)
}

func SprintColor(color Color, msg string, arg ...any) string {
	return fmt.Sprintf("%s%s%s", color, fmt.Sprintf(msg, arg...), ColorReset)
}

func PrintTable(header []string, rows []table.Row) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	headerRow := make(table.Row, len(header))
	for i, h := range header {
		headerRow[i] = h
	}
	t.AppendHeader(headerRow)
	t.AppendRows(rows)
	t.Render()
}
