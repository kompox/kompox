package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// Doc holds front matter and path info for a design document.
type Doc struct {
	Title    string `yaml:"title"`
	Version  string `yaml:"version"`
	Status   string `yaml:"status"`
	Updated  string `yaml:"updated"`
	Language string `yaml:"language"`

	RelPath string `yaml:"-"`
}

// Group groups docs by version bucket for rendering.
type Group struct {
	Name string
	Key  string
	Docs []Doc
}

// IndexData is the template data root.
type IndexData struct {
	Title    string
	Language string
	Updated  string
	Groups   []Group
	Labels   TemplateLabels
}

// TemplateLabels holds localized labels loaded from the template front matter.
type TemplateLabels struct {
	Groups map[string]string `yaml:"groups"`
}

func main() {
	var (
		designDir = flag.String("design-dir", ".", "Root directory to scan for design markdown files")
		output    = flag.String("output", "", "Output index markdown path (defaults to README[.<lang>].md under design-dir)")
		tplPath   = flag.String("template", "", "Go text template path for index generation (defaults to gen/README.<lang>.md under design-dir)")
		lang      = flag.String("lang", "en", "UI language (used for UI labels)")
		noFilter  = flag.Bool("no-lang-filter", true, "Include all languages by default (set to false to filter by -lang)")
	)
	flag.Parse()

	// Normalize options for relative execution (e.g., running inside design directory).
	if strings.TrimSpace(*designDir) == "" {
		*designDir = "."
	}
	langLower := strings.ToLower(strings.TrimSpace(*lang))
	if langLower == "" {
		langLower = "en"
		*lang = "en"
	}

	if strings.TrimSpace(*tplPath) == "" {
		candidate := defaultTemplatePath(*designDir, langLower)
		if _, err := os.Stat(candidate); err != nil {
			// Fallback to English template if language-specific one is absent.
			candidate = filepath.Join(*designDir, "gen", "README.en.md")
			if _, err := os.Stat(candidate); err != nil {
				// As a final fallback, try the Japanese template for backward compatibility.
				candidate = filepath.Join(*designDir, "gen", "README.ja.md")
			}
		}
		*tplPath = candidate
	}

	if strings.TrimSpace(*output) == "" {
		*output = defaultOutputPath(*designDir, langLower)
	}

	docs, err := scanDocs(*designDir, *lang, *output, *tplPath, !*noFilter)
	if err != nil {
		exitErr(err)
	}
	// Load labels from template front matter (optional).
	labels, err := parseTemplateLabels(*tplPath)
	if err != nil {
		exitErr(fmt.Errorf("parse template labels: %w", err))
	}

	groups := groupDocs(docs, labels)

	tplBytes, err := os.ReadFile(*tplPath)
	if err != nil {
		exitErr(fmt.Errorf("read template: %w", err))
	}
	tpl, err := template.New(filepath.Base(*tplPath)).Parse(string(tplBytes))
	if err != nil {
		exitErr(fmt.Errorf("parse template: %w", err))
	}

	now := time.Now().Format("2006-01-02")
	data := IndexData{
		Title:    "Kompox Design Docs Index",
		Language: *lang,
		Updated:  now,
		Groups:   groups,
		Labels:   labels,
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		exitErr(fmt.Errorf("execute template: %w", err))
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		exitErr(fmt.Errorf("ensure output dir: %w", err))
	}
	if err := os.WriteFile(*output, buf.Bytes(), 0o644); err != nil {
		exitErr(fmt.Errorf("write output: %w", err))
	}
}

func defaultTemplatePath(designDir, lang string) string {
	if lang == "en" {
		return filepath.Join(designDir, "gen", "README.en.md")
	}
	return filepath.Join(designDir, "gen", fmt.Sprintf("README.%s.md", lang))
}

func defaultOutputPath(designDir, lang string) string {
	if lang == "en" {
		return filepath.Join(designDir, "README.md")
	}
	return filepath.Join(designDir, fmt.Sprintf("README.%s.md", lang))
}

func scanDocs(root, lang, outputPath, tplPath string, filterByLang bool) ([]Doc, error) {
	var docs []Doc

	absOut, _ := filepath.Abs(outputPath)
	absTpl, _ := filepath.Abs(tplPath)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip generator and templates directories.
		if d.IsDir() && (d.Name() == "gen" || d.Name() == "templates") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		// Skip generated index files (README*.md) from being treated as documents.
		base := strings.ToLower(d.Name())
		if base == "readme.md" || strings.HasPrefix(base, "readme.") || strings.HasPrefix(base, "index.") {
			return nil
		}

		abs, _ := filepath.Abs(path)
		// Skip output and template files.
		if abs == absOut || abs == absTpl {
			return nil
		}

		doc, ok, err := parseFrontMatter(path)
		if err != nil {
			return fmt.Errorf("parse front matter for %s: %w", path, err)
		}
		if !ok {
			// No front matter; skip.
			return nil
		}
		// Filter by language when requested and the document declares a language.
		if filterByLang {
			if strings.TrimSpace(doc.Language) != "" && strings.TrimSpace(strings.ToLower(doc.Language)) != strings.ToLower(lang) {
				return nil
			}
		}
		// Exclude meta docs from index.
		if strings.EqualFold(doc.Version, "meta") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
		doc.RelPath = "./" + rel

		// Fallbacks and normalization.
		if strings.TrimSpace(doc.Title) == "" {
			doc.Title = titleFromFilename(d.Name())
		}
		doc.Updated = normalizeDate(doc.Updated)

		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Stable sort by title within entire set; grouping will preserve this order.
	sort.SliceStable(docs, func(i, j int) bool {
		ti := strings.ToLower(docs[i].Title)
		tj := strings.ToLower(docs[j].Title)
		if ti == tj {
			return docs[i].RelPath < docs[j].RelPath
		}
		return ti < tj
	})
	return docs, nil
}

func parseFrontMatter(path string) (Doc, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, false, err
	}
	s := string(b)
	s = trimBOM(s)

	// Front matter is between first line '---' and the next line '---'.
	lines := splitLinesLF(s)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Doc{}, false, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return Doc{}, false, errors.New("unterminated front matter: missing closing ---")
	}
	yamlText := strings.Join(lines[1:end], "\n")
	var doc Doc
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil {
		return Doc{}, false, err
	}
	return doc, true, nil
}

func groupDocs(docs []Doc, labels TemplateLabels) []Group {
	orderKeys := []string{"v1", "v2", "pub", "other"}
	buckets := map[string][]Doc{
		"v1":    {},
		"v2":    {},
		"pub":   {},
		"other": {},
	}
	for _, d := range docs {
		key := strings.ToLower(strings.TrimSpace(d.Version))
		switch key {
		case "v1", "v2", "pub":
			buckets[key] = append(buckets[key], d)
		default:
			buckets["other"] = append(buckets["other"], d)
		}
	}
	var groups []Group
	for _, key := range orderKeys {
		if len(buckets[key]) == 0 {
			continue
		}
		groups = append(groups, Group{
			Key:  key,
			Name: groupName(key, labels),
			Docs: buckets[key],
		})
	}
	return groups
}

// parseTemplateLabels extracts the front matter from the template markdown file
// and returns the Labels section. It sanitizes template expressions like {{ .Var }}
// so YAML parsing does not fail.
func parseTemplateLabels(tplPath string) (TemplateLabels, error) {
	b, err := os.ReadFile(tplPath)
	if err != nil {
		return TemplateLabels{}, err
	}
	s := string(b)
	s = trimBOM(s)
	lines := splitLinesLF(s)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		// No front matter, return empty labels
		return TemplateLabels{}, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return TemplateLabels{}, errors.New("template front matter not closed with '---'")
	}
	// Extract only the 'labels:' subtree to avoid parsing other fields that may contain Go templates.
	start := -1
	for i := 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "labels:" {
			start = i
			break
		}
	}
	if start == -1 {
		return TemplateLabels{}, nil
	}
	indent := leadingSpaces(lines[start])
	var subtree []string
	subtree = append(subtree, "labels:")
	for i := start + 1; i < end; i++ {
		ln := lines[i]
		if strings.TrimSpace(ln) != "" && leadingSpaces(ln) <= indent {
			break
		}
		subtree = append(subtree, ln)
	}
	yamlText := strings.Join(subtree, "\n")

	type fm struct {
		Labels TemplateLabels `yaml:"labels"`
	}
	var F fm
	if err := yaml.Unmarshal([]byte(yamlText), &F); err != nil {
		return TemplateLabels{}, err
	}
	return F.Labels, nil
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func groupName(key string, labels TemplateLabels) string {
	if labels.Groups != nil {
		if v, ok := labels.Groups[key]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return key
}

func titleFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.ReplaceAll(base, "-", " ")
	return base
}

func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006/01/02",
		"2006-1-2",
		"2006/1/2",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	// Keep as-is if unknown format.
	return s
}

func splitLinesLF(s string) []string {
	// Normalize CRLF to LF.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func trimBOM(s string) string {
	if len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF {
		return s[3:]
	}
	return s
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "design-gen: %v\n", err)
	os.Exit(1)
}
