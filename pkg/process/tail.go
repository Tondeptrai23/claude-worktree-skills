package process

// LineLogger formats text with color
type LineLogger func(color interface{}, args ...any)

// TailFiles tails multiple log files with color decoration.
// The logger callback is used to format colored output (implementation in cmd layer).
func TailFiles(logFiles []string, fileColors map[string]interface{}, logger LineLogger) error {
	return tailFilesImpl(logFiles, fileColors, logger)
}
