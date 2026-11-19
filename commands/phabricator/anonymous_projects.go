package phabricator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	projectPHIDPattern        = regexp.MustCompile(`^PHID-PROJ-[a-z0-9]+$`)
	anonymousProjectSlug      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	anonymousProjectProfile   = regexp.MustCompile(`^/project/profile/([1-9][0-9]*)/$`)
	anonymousProjectResultURL = regexp.MustCompile(`^/project/view/([1-9][0-9]*)/$`)
)

func (c anonymousPhabricator) project(ctx context.Context, ref string) (*phabProject, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "#")
	if ref == "" {
		return nil, fmt.Errorf("project is required")
	}
	if projectPHIDPattern.MatchString(ref) {
		return nil, fmt.Errorf("public project lookup takes a tag, exact name or numeric ID; use the PHID directly with task list --project")
	}
	if id, err := strconv.Atoi(ref); err == nil && id > 0 {
		return c.projectByPath(ctx, "/project/profile/"+strconv.Itoa(id)+"/")
	}
	if anonymousProjectSlug.MatchString(ref) {
		project, err := c.projectByPath(ctx, "/tag/"+ref+"/")
		if !errors.Is(err, errAnonymousNotVisible) {
			return project, err
		}
	}

	params := url.Values{"name": {ref}}
	var match string
	seen := map[string]bool{}
	for {
		doc, err := c.get(ctx, "/project/", params)
		if err != nil {
			return nil, err
		}
		var ambiguous bool
		doc.Find(".phui-oi-list-view .phui-oi-link").Each(func(_ int, link *goquery.Selection) {
			if !strings.EqualFold(strings.TrimSpace(link.AttrOr("title", link.Text())), ref) {
				return
			}
			parts := anonymousProjectResultURL.FindStringSubmatch(link.AttrOr("href", ""))
			if parts == nil {
				return
			}
			if match != "" && match != parts[1] {
				ambiguous = true
			}
			match = parts[1]
		})
		if ambiguous {
			return nil, fmt.Errorf("multiple public projects named %q; use a numeric project ID", ref)
		}
		cursor := anonymousPageCursor(doc, 0)
		if cursor.After == nil {
			break
		}
		next := *cursor.After
		if seen[next] {
			return nil, fmt.Errorf("public Phabricator project search repeated its cursor")
		}
		seen[next] = true
		params.Set("after", next)
	}
	if match == "" {
		return nil, fmt.Errorf("public Phabricator project %q not found", ref)
	}
	return c.projectByPath(ctx, "/project/profile/"+match+"/")
}

func (c anonymousPhabricator) projectByPath(ctx context.Context, path string) (*phabProject, error) {
	doc, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	nav := doc.Find(".phabricator-nav-local")
	link := nav.Find("a[href^='/project/profile/']").First()
	profile := link.AttrOr("href", "")
	parts := anonymousProjectProfile.FindStringSubmatch(profile)
	if parts == nil || (strings.HasPrefix(path, "/project/profile/") && path != profile) {
		return nil, fmt.Errorf("project %s: %w", path, errAnonymousNotVisible)
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid public Phabricator project ID: %w", err)
	}
	name := strings.TrimSpace(link.Text())
	taskLink, err := url.Parse(nav.Find("a[href^='/maniphest/?project=']").First().AttrOr("href", ""))
	if err != nil {
		return nil, fmt.Errorf("invalid public Phabricator project task link: %w", err)
	}
	phid := taskLink.Query().Get("project")
	if name == "" || !projectPHIDPattern.MatchString(phid) {
		return nil, fmt.Errorf("could not parse public project %s", path)
	}
	project := &phabProject{ID: id, PHID: phid, URI: defaultPhabricatorURL + profile}
	project.Fields.Name = name
	return project, nil
}
