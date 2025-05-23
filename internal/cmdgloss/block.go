package cmdgloss

import (
	"fmt"
	"strings"
)

func SuccessHeading(text string) string {
	return strings.TrimSpace(text) + " 🎉"
}

func ThreePartBlock(heading string, details map[string]string, footer string) string {
	lines := []string{}

	if len(heading) > 0 {
		lines = append(lines, heading)
	}
	if len(details) > 0 {
		lines = append(lines, "")
		for key, value := range details {
			lines = append(lines, key+": "+value)
		}
	}

	if len(footer) > 0 {
		lines = append(lines, "", footer)
	}

	return Block(lines)
}

func PrintThreePartBlock(heading string, details map[string]string, footer string) {
	fmt.Print(ThreePartBlock(heading, details, footer))
}

func Block(lines []string) string {
	block := ""
	block = block + "***************************************\n"
	for _, line := range lines {
		block = block + line + "\n"
	}
	block = block + "***************************************\n"
	return block
}

func PrintBlock(lines []string) {
	fmt.Print(Block(lines))
}
