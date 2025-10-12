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

// Doc holds front matter and path info for a design document.
type Doc struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Version  string `yaml:"version"`
	Status   string `yaml:"status"`
	Updated  string `yaml:"updated"`
	Language string `yaml:"language"`

	RelPath string `yaml:"-"`
}

// Group represents a set of documents bucketed by an inferred key (first directory segment).
type Group struct {
	Key  string
	Docs []Doc
}

// IndexData is the template data root.
type IndexData struct {
	Title    string
	Language string
	Updated  string
	Groups   []Group
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

	// Normalize options for relative execution.
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
				// Final fallback: Japanese template.
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
	tplBytes, err := os.ReadFile(*tplPath)
	if err != nil {
		exitErr(fmt.Errorf("read template: %w", err))
	}
	tpl, err := template.New(filepath.Base(*tplPath)).Parse(string(tplBytes))
	if err != nil {
		exitErr(fmt.Errorf("parse template: %w", err))
	}

	// Determine group order from a named template block if present (template-defined), otherwise default.
	order := evalGroupOrder(tpl)
	groups := groupDocs(docs, order)

	now := time.Now().Format("2006-01-02")
	data := IndexData{
		Title:    "Kompox Design Docs Index",
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

		// Fill missing display fields and normalize date.
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

func groupDocs(docs []Doc, orderKeys []string) []Group {
	// Bucket documents by folder-based key inferred from RelPath.
	buckets := map[string][]Doc{}
	for _, d := range docs {
		key := inferGroupKeyFromPath(d.RelPath)
		buckets[key] = append(buckets[key], d)
	}

	// Use order passed from template; fallback to a sensible default.
	if len(orderKeys) == 0 {
		orderKeys = []string{"v1", "v2", "adr", "pub", "other"}
	}

	used := map[string]bool{}
	var groups []Group
	for _, key := range orderKeys {
		if docs := buckets[key]; len(docs) > 0 {
			// Apply per-group sorting rules before appending.
			if strings.EqualFold(key, "adr") {
				sort.SliceStable(docs, func(i, j int) bool {
					ni := adrNumberFromDoc(docs[i])
					nj := adrNumberFromDoc(docs[j])
					if ni == nj {
						return docs[i].RelPath < docs[j].RelPath
					}
					return ni < nj
				})
			}
			groups = append(groups, Group{Key: key, Docs: docs})
			used[key] = true
		}
	}
	// Append any remaining buckets in alphabetical order.
	var rest []string
	for k := range buckets {
		if !used[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		ds := buckets[k]
		if strings.EqualFold(k, "adr") {
			sort.SliceStable(ds, func(i, j int) bool {
				ni := adrNumberFromDoc(ds[i])
				nj := adrNumberFromDoc(ds[j])
				if ni == nj {
					return ds[i].RelPath < ds[j].RelPath
				}
				return ni < nj
			})
		}
		groups = append(groups, Group{Key: k, Docs: ds})
	}
	return groups
}

// adrNumberFromDoc extracts the numeric ADR id (e.g., 6 from "K4x-ADR-006").
// Returns a large number when it cannot extract, so unknowns go last.
func adrNumberFromDoc(d Doc) int {
	// Prefer front-matter id.
	if n, ok := adrNumberFromString(d.ID); ok {
		return n
	}
	// Fallback to filename.
	base := filepath.Base(d.RelPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if n, ok := adrNumberFromString(stem); ok {
		return n
	}
	return 1 << 30
}

func adrNumberFromString(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	up := strings.ToUpper(s)
	idx := strings.LastIndex(up, "ADR-")
	if idx < 0 {
		return 0, false
	}
	numPart := s[idx+4:]
	// take leading digits only
	end := 0
	for end < len(numPart) {
		c := numPart[end]
		if c < '0' || c > '9' {
			break
		}
		end++
	}
	if end == 0 {
		return 0, false
	}
	numPart = numPart[:end]
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return 0, false
	}
	return n, true
}

// inferGroupKeyFromPath returns first directory segment as a grouping key.
func inferGroupKeyFromPath(rel string) string {
	rp := strings.TrimPrefix(strings.ToLower(rel), "./")
	parts := strings.SplitN(rp, "/", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "other"
	}
	// Return the first segment as key; templates define names/order. Unknowns fall back to 'other' only if empty.
	return parts[0]
}

// evalGroupOrder executes a named sub-template "groupOrder" if present and
// returns its content as a comma-separated list of keys (lowercased, trimmed).
func evalGroupOrder(t *template.Template) []string {
	sub := t.Lookup("groupOrder")
	if sub == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := sub.Execute(&buf, nil); err != nil {
		return nil
	}
	s := trimBOM(buf.String())
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		k := strings.ToLower(strings.TrimSpace(p))
		if k != "" {
			out = append(out, k)
		}
	}
	return out
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
