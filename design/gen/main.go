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

// IndexData is the template data root for each category index.
type IndexData struct {
	Title    string
	Category string
	Updated  string
	Docs     []Doc
}

// CategoryTemplate maps a category to its template file.
type CategoryTemplate struct {
	Category     string
	TemplatePath string
}

func main() {
	var (
		designDir = flag.String("design-dir", ".", "Design directory root")
	)
	flag.Parse()

	if strings.TrimSpace(*designDir) == "" {
		*designDir = "."
	}

	categoryTemplates, err := discoverCategoryTemplates(filepath.Join(*designDir, "gen"))
	if err != nil {
		exitErr(err)
	}
	if len(categoryTemplates) == 0 {
		exitErr(fmt.Errorf("no category templates found under %s", filepath.Join(*designDir, "gen", "*", "README.md")))
	}

	now := time.Now().Format("2006-01-02")

	for _, ct := range categoryTemplates {
		outDir := filepath.Join(*designDir, ct.Category)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			exitErr(fmt.Errorf("ensure category dir (%s): %w", ct.Category, err))
		}
		outputPath := filepath.Join(outDir, "README.md")

		docs, err := scanCategoryDocs(outDir)
		if err != nil {
			exitErr(fmt.Errorf("scan category docs (%s): %w", ct.Category, err))
		}

		if strings.EqualFold(ct.Category, "adr") {
			sort.SliceStable(docs, func(i, j int) bool {
				ni := adrNumberFromDoc(docs[i])
				nj := adrNumberFromDoc(docs[j])
				if ni == nj {
					return docs[i].RelPath < docs[j].RelPath
				}
				return ni < nj
			})
		} else {
			sort.SliceStable(docs, func(i, j int) bool {
				return docs[i].RelPath < docs[j].RelPath
			})
		}

		data := IndexData{
			Title:    fmt.Sprintf("Kompox Design %s Index", strings.ToUpper(ct.Category)),
			Category: ct.Category,
			Updated:  now,
			Docs:     docs,
		}

		if err := renderTemplateToFile(ct.TemplatePath, outputPath, data); err != nil {
			exitErr(fmt.Errorf("render category index (%s): %w", ct.Category, err))
		}
	}
}

func discoverCategoryTemplates(genDir string) ([]CategoryTemplate, error) {
	entries, err := os.ReadDir(genDir)
	if err != nil {
		return nil, fmt.Errorf("read gen dir: %w", err)
	}

	var out []CategoryTemplate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		category := entry.Name()
		tplPath := filepath.Join(genDir, category, "README.md")
		if _, err := os.Stat(tplPath); err != nil {
			continue
		}
		out = append(out, CategoryTemplate{
			Category:     category,
			TemplatePath: tplPath,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Category < out[j].Category
	})

	return out, nil
}

func renderTemplateToFile(templatePath, outputPath string, data any) error {
	if strings.TrimSpace(templatePath) == "" {
		return errors.New("template path is required")
	}

	tplBytes, readErr := os.ReadFile(templatePath)
	if readErr != nil {
		return fmt.Errorf("read template: %w", readErr)
	}
	tpl, err := template.New(filepath.Base(templatePath)).Parse(string(tplBytes))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func scanCategoryDocs(categoryDir string) ([]Doc, error) {
	var docs []Doc

	if _, err := os.Stat(categoryDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return docs, nil
		}
		return nil, err
	}

	err := filepath.WalkDir(categoryDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		base := strings.ToLower(d.Name())
		if base == "readme.md" || strings.HasPrefix(base, "readme.") || strings.HasPrefix(base, "index.") {
			return nil
		}
		if base == "guide.md" || strings.HasPrefix(base, "guide.") {
			return nil
		}

		doc, ok, parseErr := parseFrontMatter(path)
		if parseErr != nil {
			return fmt.Errorf("parse front matter for %s: %w", path, parseErr)
		}
		if !ok {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(doc.Version), "meta") {
			return nil
		}

		rel, relErr := filepath.Rel(categoryDir, path)
		if relErr != nil {
			return relErr
		}
		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
		doc.RelPath = "./" + rel

		if strings.TrimSpace(doc.ID) == "" {
			doc.ID = strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		}
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
