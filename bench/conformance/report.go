package main

import (
	"bufio"
	"cmp"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// count is a key's number of tests, with examples.
type count struct {
	key      string
	n        int
	sole     int // tests it alone blocks, for unsupported features
	examples []string
}

// countKeys counts the results of the category by key.
func countKeys(rs []*result, category string) []*count {
	byKey := map[string]*count{}
	for _, r := range rs {
		if r.category != category {
			continue
		}
		keys := r.keys
		if len(keys) == 0 {
			keys = []string{r.key}
		}
		for _, k := range keys {
			c := byKey[k]
			if c == nil {
				c = &count{key: k}
				byKey[k] = c
			}
			c.n++
			if len(keys) == 1 {
				c.sole++
			}
			if len(c.examples) < 2 {
				c.examples = append(c.examples, r.path)
			}
		}
	}
	cs := make([]*count, 0, len(byKey))
	for _, c := range byKey {
		cs = append(cs, c)
	}
	slices.SortFunc(cs, func(a, b *count) int {
		if a.n != b.n {
			return b.n - a.n
		}
		return strings.Compare(a.key, b.key)
	})
	return cs
}

// reportKeys writes a table of the category's keys, at most limit rows.
func reportKeys(sb *strings.Builder, rs []*result, category, keyName string, limit int, sole bool) {
	cs := countKeys(rs, category)
	if len(cs) == 0 {
		return
	}
	if sole {
		fmt.Fprintf(sb, "| %s | Tests | Only blocker | Examples |\n|:---|---:|---:|:---|\n", keyName)
	} else {
		fmt.Fprintf(sb, "| %s | Tests | Examples |\n|:---|---:|:---|\n", keyName)
	}
	for i, c := range cs {
		if i == limit {
			rest := 0
			for _, c := range cs[i:] {
				rest += c.n
			}
			fmt.Fprintf(sb, "| *%d more* | %d | |\n", len(cs)-i, rest)
			break
		}
		ex := make([]string, len(c.examples))
		for j, e := range c.examples {
			ex[j] = "`" + mdEscape(e) + "`"
		}
		if sole {
			fmt.Fprintf(sb, "| %s | %d | %d | %s |\n", cell(c.key), c.n, c.sole, strings.Join(ex, ", "))
		} else {
			fmt.Fprintf(sb, "| %s | %d | %s |\n", cell(c.key), c.n, strings.Join(ex, ", "))
		}
	}
	fmt.Fprintln(sb)
}

// reportGreedy writes the order in which adding the unsupported features
// unblocks the most tests: at each step, the feature that unblocks the
// most tests given the ones before it. (As bench/survey's, for tests.)
// A test is unblocked when all of the features found in it are added;
// the parser stops at the first error of each declaration, so a test can
// need more.
func reportGreedy(sb *strings.Builder, rs []*result, steps int) {
	var blocked [][]string
	for _, r := range rs {
		if r.category == catUnsupported {
			blocked = append(blocked, r.keys)
		}
	}
	cs := countKeys(rs, catUnsupported)
	if len(cs) == 0 {
		return
	}
	unblocked := func(added map[string]bool) int {
		n := 0
		for _, keys := range blocked {
			if !slices.ContainsFunc(keys, func(k string) bool { return !added[k] }) {
				n++
			}
		}
		return n
	}
	fmt.Fprintf(sb, "| Step | Feature | Tests unblocked | Total |\n|---:|:---|---:|---:|\n")
	added := map[string]bool{}
	total := 0
	for step := 1; step <= steps && step <= len(cs); step++ {
		best, bestGain, bestN := "", -1, -1
		for _, c := range cs {
			if added[c.key] {
				continue
			}
			added[c.key] = true
			gain := unblocked(added) - total
			delete(added, c.key)
			if gain > bestGain || gain == bestGain && c.n > bestN {
				best, bestGain, bestN = c.key, gain, c.n
			}
		}
		if best == "" {
			break
		}
		added[best] = true
		total += bestGain
		fmt.Fprintf(sb, "| %d | %s | %d | %d |\n", step, cell(best), bestGain, total)
	}
	fmt.Fprintln(sb)
}

// byCategory counts the results by category.
func byCategory(rs []*result) map[string]int {
	m := map[string]int{}
	for _, r := range rs {
		m[r.category]++
	}
	return m
}

// topDir returns the first directory of a test's path, or "." for a test
// at the top.
func topDir(path string) string {
	if d, _, ok := strings.Cut(path, "/"); ok {
		return d
	}
	return "."
}

// writeTSV writes one line per result: path, category, key (all the
// features, for parse/unsupported) and detail.
func writeTSV(path string, rs []*result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "path\tcategory\tkey\tdetail")
	sorted := slices.Clone(rs)
	slices.SortFunc(sorted, func(a, b *result) int { return cmp.Compare(a.path, b.path) })
	for _, r := range sorted {
		key := r.key
		if len(r.keys) > 0 {
			key = strings.Join(r.keys, "; ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.path, r.category, tsvField(key), tsvField(r.detail))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// addressRE matches what differs from run to run in a JVM message: a
// hidden class's address and an object's identity hash.
var addressRE = regexp.MustCompile(`/0x[0-9a-f]+|@[0-9a-f]+\b`)

func tsvField(s string) string {
	s = addressRE.ReplaceAllString(s, "")
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// compareBaseline prints the tests whose category changed from or to a
// good one since the baseline TSV: regressions, then fixes.
func compareBaseline(path string, rs []*result, good func(string) bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	old := map[string]string{}
	for _, line := range strings.Split(string(data), "\n")[1:] {
		f := strings.SplitN(line, "\t", 4)
		if len(f) >= 2 {
			old[f[0]] = f[1]
		}
	}
	var worse, better []string
	for _, r := range rs {
		o, ok := old[r.path]
		if !ok {
			continue
		}
		switch {
		case good(o) && !good(r.category):
			worse = append(worse, fmt.Sprintf("%s: %s, now %s (%s)", r.path, o, r.category, r.key))
		case !good(o) && good(r.category):
			better = append(better, fmt.Sprintf("%s: %s, now %s", r.path, o, r.category))
		}
	}
	slices.Sort(worse)
	slices.Sort(better)
	fmt.Printf("baseline %s: %d regressions, %d fixed\n", path, len(worse), len(better))
	for _, s := range worse {
		fmt.Println("  regression:", s)
	}
	for _, s := range better {
		fmt.Println("  fixed:", s)
	}
	return nil
}

// mdEscape escapes the characters of s that a Markdown table cell can't
// hold as they are.
func mdEscape(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ", "`", "'").Replace(s)
}

// cell escapes text for a table cell, as code.
func cell(s string) string {
	return "`" + strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "`", "'") + "`"
}

func pct(n, total int) string {
	if total == 0 {
		return "–"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(total))
}
