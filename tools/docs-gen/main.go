// Command docs-gen builds and synchronizes the CLI documentation.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	mode := flag.String("mode", "generate", "generate, manual, review, check, audit, pull, publish, import, or accept")
	output := flag.String("output", "website/src/content/docs/manual", "Generated manual directory (manual mode)")
	page := flag.String("page", "", "Wiki title for import or accept")
	source := flag.String("source", "", "Markdown path within docs/ for import")
	flag.Parse()
	if *mode == "manual" {
		if err := generateManual(*output); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(*mode, *page, *source); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode, title, source string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	switch mode {
	case "review":
		return serveDocumentationReview(config)
	case "generate":
		return generate(config)
	case "audit":
		return auditDocumentation(config)
	case "import":
		return importGuide(config, title, source)
	case "accept":
		return acceptReview(config, title)
	case "check", "pull", "publish":
		return synchronize(config, mode)
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}
}
