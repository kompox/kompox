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
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// TaskDoc represents a maintainer task document discovered under _dev/tasks.
// It maps YAML front matter fields and holds path metadata.
type TaskDoc struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Status   string `yaml:"status"`
	Updated  string `yaml:"updated"`
	Language string `yaml:"language"`
	Category string `yaml:"category"`
	Owner    string `yaml:"owner"`

	RelPath string `yaml:"-"`
}

// YearGroup groups tasks by year key (e.g., "2025").
type YearGroup struct {
	Key  string
	Docs []TaskDoc
}

// IndexData is the template data root.
type IndexData struct {
	Title    string
	Language string
	Updated  string
	Groups   []YearGroup
}

func main() {
	var (
		tasksDir = flag.String("tasks-dir", ".", "Root directory to scan for task markdown files")
		output   = flag.String("output", "", "Output index markdown path (defaults to README[.<lang>].md under tasks-dir)")
		tplPath  = flag.String("template", "", "Go text template path for index generation (defaults to README.<lang>.md under tasks-dir)")
		lang     = flag.String("lang", "en", "UI language (used for UI labels)")
		noFilter = flag.Bool("no-lang-filter", true, "Include all languages by default (set to false to filter by -lang)")
	)
	flag.Parse()

	if strings.TrimSpace(*tasksDir) == "" {
		*tasksDir = "."
	}
	langLower := strings.ToLower(strings.TrimSpace(*lang))
	if langLower == "" {
		langLower = "en"
		*lang = "en"
	}

	if strings.TrimSpace(*tplPath) == "" {
		candidate := defaultTemplatePath(*tasksDir, langLower)
		if _, err := os.Stat(candidate); err != nil {
			// Fallback to English template if language-specific one is absent.
			candidate = filepath.Join(*tasksDir, "gen", "README.en.md")
			if _, err := os.Stat(candidate); err != nil {
				// Final fallback: Japanese template.
				candidate = filepath.Join(*tasksDir, "gen", "README.ja.md")
			}
		}
		*tplPath = candidate
	}

	if strings.TrimSpace(*output) == "" {
		*output = defaultOutputPath(*tasksDir, langLower)
	}

	docs, err := scanDocs(*tasksDir, *lang, *output, *tplPath, !*noFilter)
	if err != nil {
		exitErr(err)
	}
	tplBytes, err := os.ReadFile(*tplPath)
	if err != nil {
		exitErr(fmt.Errorf("read template: %w", err))
	}
	tpl, err := template.New(filepath.Base(*tplPath)).Parse(string(tplBytes))
	if err != nil {
		exitErr(fmt.Errorf("parse template: %w", err))
	}

	groups := groupByYear(docs)

	now := time.Now().Format("2006-01-02")
	data := IndexData{
		Title:    "Maintainer Tasks Index",
		Language: *lang,
		Updated:  now,
		Groups:   groups,
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

func defaultTemplatePath(dir, lang string) string {
	if lang == "en" {
		return filepath.Join(dir, "gen", "README.en.md")
	}
	return filepath.Join(dir, "gen", fmt.Sprintf("README.%s.md", lang))
}

func defaultOutputPath(dir, lang string) string {
	if lang == "en" {
		return filepath.Join(dir, "README.md")
	}
	return filepath.Join(dir, fmt.Sprintf("README.%s.md", lang))
}

func scanDocs(root, lang, outputPath, tplPath string, filterByLang bool) ([]TaskDoc, error) {
	var docs []TaskDoc

	absOut, _ := filepath.Abs(outputPath)
	absTpl, _ := filepath.Abs(tplPath)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip generator directory if present
			if d.Name() == "gen" {
				return filepath.SkipDir
			}
			return nil
		}
		nameLower := strings.ToLower(d.Name())
		if !strings.HasSuffix(nameLower, ".md") {
			return nil
		}
		// Skip readme and guide/template files
		if nameLower == "readme.md" || strings.HasPrefix(nameLower, "readme.") || strings.HasPrefix(nameLower, "guide.") || strings.HasPrefix(nameLower, "index.") {
			return nil
		}

		abs, _ := filepath.Abs(path)
		if abs == absOut || abs == absTpl {
			return nil
		}

		doc, ok, err := parseFrontMatter(path)
		if err != nil {
			return fmt.Errorf("parse front matter for %s: %w", path, err)
		}
		if !ok {
			// Require front matter for tasks to be indexed.
			return nil
		}
		if filterByLang {
			if strings.TrimSpace(doc.Language) != "" && strings.ToLower(strings.TrimSpace(doc.Language)) != strings.ToLower(lang) {
				return nil
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
		doc.RelPath = "./" + rel

		// Fill missing display fields and normalize values.
		if strings.TrimSpace(doc.Title) == "" {
			doc.Title = titleFromFilename(d.Name())
		}
		if strings.TrimSpace(doc.ID) == "" {
			doc.ID = idFromFilename(d.Name())
		}
		doc.Updated = normalizeDate(doc.Updated)

		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by Updated desc, then Title asc.
	sort.SliceStable(docs, func(i, j int) bool {
		di := docs[i]
		dj := docs[j]
		ti := parseDate(di.Updated)
		tj := parseDate(dj.Updated)
		if !ti.Equal(tj) {
			return tj.Before(ti) // newer first
		}
		return strings.ToLower(di.Title) < strings.ToLower(dj.Title)
	})
	return docs, nil
}

func parseFrontMatter(path string) (TaskDoc, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return TaskDoc{}, false, err
	}
	s := trimBOM(string(b))
	lines := splitLinesLF(s)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return TaskDoc{}, false, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return TaskDoc{}, false, errors.New("unterminated front matter: missing closing ---")
	}
	yamlText := strings.Join(lines[1:end], "\n")
	var doc TaskDoc
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil {
		return TaskDoc{}, false, err
	}
	return doc, true, nil
}

func groupByYear(docs []TaskDoc) []YearGroup {
	buckets := map[string][]TaskDoc{}
	for _, d := range docs {
		y := yearKey(d)
		buckets[y] = append(buckets[y], d)
	}

	// Collect keys and sort by numeric year desc; non-numeric keys go last alphabetically.
	type key struct {
		raw   string
		year  int
		isNum bool
	}
	var keys []key
	for k := range buckets {
		if y, err := strconv.Atoi(k); err == nil {
			keys = append(keys, key{raw: k, year: y, isNum: true})
		} else {
			keys = append(keys, key{raw: k, isNum: false})
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].isNum && keys[j].isNum {
			return keys[i].year > keys[j].year // newer year first
		}
		if keys[i].isNum != keys[j].isNum {
			return keys[i].isNum // numeric years before others
		}
		return keys[i].raw < keys[j].raw
	})

	var groups []YearGroup
	for _, k := range keys {
		groups = append(groups, YearGroup{Key: k.raw, Docs: buckets[k.raw]})
	}
	return groups
}

func yearKey(d TaskDoc) string {
	// Prefer Updated date
	if y := yearFromDate(d.Updated); y != "" {
		return y
	}
	// Fallback to ID like YYYY-MM-topic
	if y := yearFromID(d.ID); y != "" {
		return y
	}
	return "undated"
}

func yearFromDate(s string) string {
	t := parseDate(s)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006")
}

func yearFromID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) >= 4 {
		if _, err := strconv.Atoi(id[0:4]); err == nil {
			return id[0:4]
		}
	}
	return ""
}

func titleFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	// Strip leading date prefix if present
	if len(base) >= 11 && base[4] == '-' && base[7] == '-' {
		base = base[11:]
	}
	base = strings.ReplaceAll(base, "-", " ")
	return base
}

func idFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	// Drop language suffix .en or .ja etc
	if idx := strings.LastIndex(base, "."); idx != -1 {
		base = base[:idx]
	}
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
	return s
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return time.Time{}
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
			return t
		}
	}
	return time.Time{}
}

func splitLinesLF(s string) []string {
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
	fmt.Fprintf(os.Stderr, "tasks-gen: %v\n", err)
	os.Exit(1)
}
