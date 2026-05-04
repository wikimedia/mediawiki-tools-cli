package output

/*
Renders grouped output inspired by ripgrep / ack:

Extension:Thanks > tests/phpunit/ApiCoreThankIntegrationTest.php
  18: * @author Addshore

Extension:Thanks > tests/phpunit/ApiCoreThankUnitTest.php
  18: * @author Addshore

Sections and items are emitted in insertion order.
*/

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

// Pretty holds the state for a grouped pretty-print run.
type Pretty struct {
	sections     map[string][]string
	sectionOrder []string
}

// PrettyBinding populates a Pretty from objects.
type PrettyBinding func(interface{}, *Pretty)

func (p *Pretty) initMap() {
	if p.sections == nil {
		p.sections = make(map[string][]string)
	}
}

// InitSection registers a section without any items.
func (p *Pretty) InitSection(name string) {
	p.initMap()
	if _, ok := p.sections[name]; !ok {
		p.sections[name] = nil
		p.sectionOrder = append(p.sectionOrder, name)
	}
}

// AddItem adds an item string to a named section, creating the section if needed.
func (p *Pretty) AddItem(section, item string) {
	p.initMap()
	if _, ok := p.sections[section]; !ok {
		p.sectionOrder = append(p.sectionOrder, section)
		p.sections[section] = nil
	}
	p.sections[section] = append(p.sections[section], item)
}

// Print renders the grouped output to writer.
// When stdout is a TTY the section headers are rendered in bold green and
// line-number prefixes (digits followed by ':') are rendered in yellow.
func (p *Pretty) Print(writer io.Writer) {
	headerFmt := color.New(color.FgGreen, color.Bold).SprintfFunc()
	lineNumFmt := color.New(color.FgYellow).SprintfFunc()

	for i, section := range p.sectionOrder {
		if i > 0 {
			fmt.Fprintln(writer)
		}

		if shouldColor() {
			fmt.Fprintln(writer, headerFmt("%s", section))
		} else {
			fmt.Fprintln(writer, section)
		}

		for _, item := range p.sections[section] {
			if shouldColor() {
				// Colour the "digits:" prefix in yellow so line numbers stand out.
				colon := strings.Index(item, ":")
				if colon > 0 && isDigitsOnly(item[:colon]) {
					fmt.Fprintf(writer, "  %s%s\n", lineNumFmt("%s:", item[:colon]), item[colon+1:])
					continue
				}
			}
			fmt.Fprintf(writer, "  %s\n", item)
		}
	}
}

// isDigitsOnly returns true when s contains only ASCII decimal digits.
func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
