// Package graph handles building and querying the knowledge graph.
package graph

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/pyra/internal/types"
	"github.com/chasedputnam/pyra/internal/util"
)

// ExtractInternalLinks extracts internal markdown links from a concept's body.
func ExtractInternalLinks(concept *types.Concept) []string {
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	matches := linkRe.FindAllStringSubmatch(concept.Body, -1)

	links := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		href := match[1]

		// Remove hash
		noHash := strings.Split(href, "#")[0]
		if noHash == "" {
			continue
		}

		// Skip external URLs
		if strings.HasPrefix(noHash, "http://") || strings.HasPrefix(noHash, "https://") || strings.HasPrefix(noHash, "//") {
			continue
		}
		if strings.HasPrefix(noHash, "mailto:") {
			continue
		}
		// Skip other protocols
		if regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`).MatchString(noHash) {
			continue
		}

		// Resolve the path
		var resolved string
		if strings.HasPrefix(noHash, "/") {
			resolved = path.Clean(noHash[1:])
		} else {
			dir := path.Dir(concept.Path)
			resolved = path.Clean(path.Join(dir, noHash))
		}

		if resolved == "" || resolved == "." {
			continue
		}

		links[util.StripMdExtension(resolved)] = true
	}

	result := make([]string, 0, len(links))
	for link := range links {
		result = append(result, link)
	}
	sort.Strings(result)
	return result
}

// BuildGraph builds a knowledge graph from a map of concepts.
func BuildGraph(conceptsByAnyKey map[string]*types.Concept) *types.KnowledgeGraph {
	// Deduplicate concepts (since map contains both ID and path keys)
	concepts := make(map[string]*types.Concept)
	for _, concept := range conceptsByAnyKey {
		concepts[concept.ID] = concept
	}

	outbound := make(map[string][]string)
	backlinks := make(map[string][]string)

	for _, concept := range concepts {
		targets := ExtractInternalLinks(concept)

		// Filter to only existing concepts
		validTargets := make([]string, 0)
		for _, target := range targets {
			if _, exists := concepts[target]; exists {
				validTargets = append(validTargets, target)
			}
		}

		outbound[concept.ID] = validTargets

		// Build backlinks
		for _, target := range validTargets {
			backlinks[target] = append(backlinks[target], concept.ID)
		}
	}

	// Sort backlinks and ensure all concepts have entries
	for id := range concepts {
		if _, ok := outbound[id]; !ok {
			outbound[id] = []string{}
		}
		if _, ok := backlinks[id]; !ok {
			backlinks[id] = []string{}
		} else {
			sort.Strings(backlinks[id])
		}
	}

	return &types.KnowledgeGraph{
		Concepts:  concepts,
		Outbound:  outbound,
		Backlinks: backlinks,
	}
}
