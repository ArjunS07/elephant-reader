package ingest

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"unicode"
)

var (
	tagRE     = regexp.MustCompile(`(?is)<[^>]+>`)
	scriptRE  = regexp.MustCompile(`(?is)<script[\s\S]*?</script>|<style[\s\S]*?</style>`)
	articleRE = regexp.MustCompile(`(?is)<article[\s\S]*?</article>`)
	mainRE    = regexp.MustCompile(`(?is)<main[\s\S]*?</main>`)
	bodyRE    = regexp.MustCompile(`(?is)<body[\s\S]*?</body>`)
	linkRE    = regexp.MustCompile(`(?is)<a\s[^>]*href=["']([^"']+)["'][^>]*>([\s\S]*?)</a>`)
	mdLinkRE  = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	codeRE    = regexp.MustCompile("(?s)```[\\s\\S]*?```")
	inlineRE  = regexp.MustCompile("`([^`]+)`")
)

func HTMLToText(source string) (markdown string, plain string) {
	fragment := source
	if match := articleRE.FindString(source); match != "" {
		fragment = match
	} else if match := mainRE.FindString(source); match != "" {
		fragment = match
	} else if match := bodyRE.FindString(source); match != "" {
		fragment = match
	}
	fragment = scriptRE.ReplaceAllString(fragment, " ")
	fragment = linkRE.ReplaceAllStringFunc(fragment, func(tag string) string {
		parts := linkRE.FindStringSubmatch(tag)
		if len(parts) < 3 {
			return tag
		}
		label := strings.TrimSpace(plainFragment(parts[2]))
		if label == "" {
			label = parts[1]
		}
		return "[" + label + "](" + parts[1] + ")"
	})
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n\n",
		"</div>", "\n\n",
		"</h1>", "\n\n",
		"</h2>", "\n\n",
		"</h3>", "\n\n",
		"</li>", "\n",
		"</blockquote>", "\n\n",
		"<li>", "- ",
		"<blockquote>", "> ",
	)
	fragment = replacer.Replace(fragment)
	fragment = regexp.MustCompile(`(?i)<h([1-6])[^>]*>`).ReplaceAllString(fragment, "${1} ")
	fragment = tagRE.ReplaceAllString(fragment, "")
	fragment = html.UnescapeString(fragment)
	lines := strings.Split(fragment, "\n")
	var cleaned []string
	blank := false
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		line = strings.TrimSpace(line)
		if line == "" {
			if !blank && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
			}
			blank = true
			continue
		}
		blank = false
		cleaned = append(cleaned, line)
	}
	markdown = strings.TrimSpace(strings.Join(cleaned, "\n"))
	plain = PlainFromMarkdown(markdown)
	return markdown, plain
}

func plainFragment(source string) string {
	source = scriptRE.ReplaceAllString(source, " ")
	source = tagRE.ReplaceAllString(source, " ")
	return strings.Join(strings.Fields(html.UnescapeString(source)), " ")
}

func PlainFromMarkdown(source string) string {
	text := codeRE.ReplaceAllString(source, " ")
	text = inlineRE.ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`!\[[^\]]*]\([^)]*\)`).ReplaceAllString(text, " ")
	text = mdLinkRE.ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^>\s?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^[-*]\s+`).ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	return strings.Join(strings.Fields(text), " ")
}

func Excerpt(plain, markdown string, metadata json.RawMessage, provided string) string {
	if strings.TrimSpace(provided) != "" {
		return strings.TrimSpace(provided)
	}
	text := strings.TrimSpace(plain)
	if text == "" {
		var meta struct {
			Kind     string `json:"kind"`
			Abstract string `json:"abstract"`
		}
		_ = json.Unmarshal(metadata, &meta)
		if meta.Kind == "paper" && strings.TrimSpace(meta.Abstract) != "" {
			text = meta.Abstract
		} else {
			text = PlainFromMarkdown(markdown)
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	sentences := splitSentences(text)
	var chosen []string
	count := 0
	limit := 2
	if len(sentences) < limit {
		limit = len(sentences)
	}
	for _, sentence := range sentences[:limit] {
		parts := strings.Fields(sentence)
		if len(chosen) == 0 && len(parts) > 35 {
			return strings.Join(parts[:32], " ")
		}
		if len(chosen) > 0 && count+len(parts) > 35 {
			break
		}
		chosen = append(chosen, sentence)
		count += len(parts)
		if count >= 25 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(chosen, " "))
}

func splitSentences(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		next := i + len(string(r))
		if next < len(text) && text[next] != ' ' {
			continue
		}
		sentence := strings.TrimSpace(text[start:next])
		if sentence != "" {
			out = append(out, sentence)
		}
		start = next
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		out = append(out, tail)
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}
