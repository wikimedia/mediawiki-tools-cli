package wikiapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// PageSelector is one finite operand. Composition retains first-operand order
// for intersections and first-seen operand order for unions.
type PageSelector struct {
	Source, Value string
	Titles        []string
}

func ParsePageSelector(value string) (PageSelector, error) {
	source, argument, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(argument) == "" {
		return PageSelector{}, fmt.Errorf("selector must be SOURCE:VALUE")
	}
	switch source {
	case "search", "category", "backlinks", "transclusions", "titles":
	default:
		return PageSelector{}, fmt.Errorf("unsupported selector %q", source)
	}
	return PageSelector{Source: source, Value: argument}, nil
}

func pageIdentity(page Page) string {
	if page.ID > 0 {
		return strconv.FormatInt(page.ID, 10)
	}
	return page.Title
}

func (c *Client) selectComposedPages(ctx context.Context, options PageSelection) ([]Page, error) {
	if options.Combine == "" {
		options.Combine = "union"
	}
	if options.Combine != "union" && options.Combine != "intersection" {
		return nil, fmt.Errorf("--combine must be union or intersection")
	}
	if options.Depth < 0 || options.Depth > 20 {
		return nil, fmt.Errorf("--depth must be between 0 and 20")
	}
	if options.MaxCategories == 0 {
		options.MaxCategories = 1000
	}
	if options.MaxCategories < 1 {
		return nil, fmt.Errorf("--max-categories must be positive")
	}
	selectors := append([]PageSelector(nil), options.Selectors...)
	if options.Source != "" {
		selectors = append([]PageSelector{{Source: options.Source, Value: options.Value, Titles: options.Titles}}, selectors...)
	}
	if len(selectors) == 0 {
		return nil, fmt.Errorf("composition requires an explicit selector")
	}
	hasCategory := false
	for _, selector := range selectors {
		if selector.Source == "category" {
			hasCategory = true
		}
	}
	if options.Depth > 0 && !hasCategory {
		return nil, fmt.Errorf("--depth requires a category selector")
	}
	result := []Page{}
	present := map[string]bool{}
	for index, selector := range selectors {
		// Enumerate complete operands before composition. Applying a per-operand
		// limit or namespace filter here would truncate intersections or hide nested
		// categories needed for traversal.
		batch, err := c.selectOperand(ctx, selector, options.Depth, options.MaxCategories)
		if err != nil {
			return nil, err
		}
		current := map[string]bool{}
		for _, page := range batch {
			key := pageIdentity(page)
			current[key] = true
			if options.Combine == "union" || index == 0 {
				if !present[key] {
					result = append(result, page)
					present[key] = true
				}
			}
		}
		if options.Combine == "intersection" && index > 0 {
			kept := []Page{}
			for _, page := range result {
				if current[pageIdentity(page)] {
					kept = append(kept, page)
				}
			}
			result = kept
		}
	}
	final := []Page{}
	for _, page := range result {
		if options.Namespace != nil && page.Namespace != *options.Namespace || options.Grep != "" && !strings.Contains(page.Title, options.Grep) {
			continue
		}
		final = append(final, page)
		if options.Limit > 0 && len(final) >= options.Limit {
			break
		}
	}
	return final, nil
}

func (c *Client) selectOperand(ctx context.Context, selector PageSelector, depth, maximum int) ([]Page, error) {
	if selector.Source != "category" || depth == 0 {
		return c.SelectPages(ctx, PageSelection{Source: selector.Source, Value: selector.Value, Titles: selector.Titles})
	}
	type entry struct {
		title string
		depth int
	}
	canonical := func(title string) string {
		if !strings.Contains(title, ":") {
			title = "Category:" + title
		}
		return strings.ReplaceAll(title, "_", " ")
	}
	queue := []entry{{canonical(selector.Value), 0}}
	visited := map[string]bool{queue[0].title: true}
	seen := map[string]bool{}
	result := []Page{}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		batch, err := c.SelectPages(ctx, PageSelection{Source: "category", Value: next.title})
		if err != nil {
			return nil, err
		}
		for _, page := range batch {
			key := pageIdentity(page)
			if !seen[key] {
				seen[key] = true
				result = append(result, page)
			}
			if page.Namespace == 14 && next.depth < depth {
				title := canonical(page.Title)
				if !visited[title] {
					if len(visited) >= maximum {
						return nil, fmt.Errorf("category traversal exceeds --max-categories %d; no partial result returned", maximum)
					}
					visited[title] = true
					queue = append(queue, entry{title, next.depth + 1})
				}
			}
		}
	}
	return result, nil
}
