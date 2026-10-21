package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/priahin-i/assistente_2/internal/domain"
	"gopkg.in/yaml.v3"
)

func renderWaitingDocument(waiting domain.Waiting) ([]byte, error) {
	meta := waitingFrontmatter{
		Kind:           kindWaiting,
		ID:             waiting.ID,
		Status:         string(waiting.Status),
		Responsible:    waiting.Responsible,
		ExpectedResult: waiting.ExpectedResult,
		NextCheck:      formatDate(waiting.NextCheck),
		Context:        string(waiting.Context),
		Tags:           waiting.Tags,
		Blocker:        waiting.Blocker,
		CreatedAt:      formatTimestamp(waiting.CreatedAt),
		UpdatedAt:      formatTimestamp(waiting.UpdatedAt),
	}
	if waiting.ResultDeadline != nil {
		meta.ResultDeadline = formatDate(*waiting.ResultDeadline)
	}
	if waiting.PromisedDeadline != nil {
		meta.PromisedDeadline = formatDate(*waiting.PromisedDeadline)
	}
	if meta.Tags == nil {
		meta.Tags = []string{}
	}

	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("Ожидаю: %s", waiting.ExpectedResult)
	body := renderWaitingBody(waitingBody{
		Title:   title,
		Notes:   waiting.Notes,
		History: waiting.CheckHistory,
	})

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(frontmatter)
	buf.WriteString("---\n\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}

func renderPersonDocument(person domain.Person) ([]byte, error) {
	meta := personFrontmatter{
		Kind:                kindPerson,
		ID:                  person.ID,
		Name:                person.Name,
		Relation:            string(person.Relation),
		DefaultCheckCadence: string(person.DefaultCheckCadence),
		Contact:             person.Contact,
		Active:              person.Active,
		CreatedAt:           formatTimestamp(person.CreatedAt),
		UpdatedAt:           formatTimestamp(person.UpdatedAt),
	}

	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return nil, err
	}

	body := renderPersonBody(personBody{
		Title: person.Name,
		Notes: person.Notes,
	})

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(frontmatter)
	buf.WriteString("---\n\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}

// Versioned, escaped comment delimiters keep arbitrary Markdown separate from
// structural headings. Legacy documents remain readable without migration.
const bodyV2 = "<!-- waiting-mcp:body-v2 -->"

func escapedPayload(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "\\") || strings.HasPrefix(line, "<!-- waiting-mcp:") {
			lines[i] = "\\" + line
		}
	}
	return strings.Join(lines, "\n")
}

func payload(name, value string) string {
	return "<!-- waiting-mcp:" + name + " -->\n" + escapedPayload(value) + "\n<!-- waiting-mcp:/" + name + " -->\n"
}

func takePayload(content, name string) (string, string, error) {
	start := "<!-- waiting-mcp:" + name + " -->\n"
	end := "\n<!-- waiting-mcp:/" + name + " -->"
	if !strings.HasPrefix(content, start) {
		return "", "", fmt.Errorf("%w: missing %s block", domain.ErrInvalidInput, name)
	}
	value, rest, ok := strings.Cut(strings.TrimPrefix(content, start), end)
	if !ok {
		return "", "", fmt.Errorf("%w: unclosed %s block", domain.ErrInvalidInput, name)
	}
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "\\\\") || strings.HasPrefix(line, "\\<!-- waiting-mcp:") {
			lines[i] = line[1:]
		}
	}
	return strings.Join(lines, "\n"), rest, nil
}

func renderWaitingBody(body waitingBody) string {
	var buf strings.Builder
	buf.WriteString(bodyV2 + "\n# " + strings.ReplaceAll(body.Title, "\n", " ") + "\n\n## Notes\n\n")
	buf.WriteString(payload("notes", body.Notes))
	buf.WriteString("\n## Check history\n\n")
	for _, record := range body.History {
		buf.WriteString("<!-- waiting-mcp:check -->\n### " + formatDate(record.Date) + " — " + record.Action + "\n\n")
		buf.WriteString(payload("message", record.Message))
		buf.WriteString("\n")
	}
	return buf.String()
}

func renderPersonBody(body personBody) string {
	return bodyV2 + "\n# " + strings.ReplaceAll(body.Title, "\n", " ") + "\n\n## Notes\n\n" + payload("notes", body.Notes)
}

func parseV2Notes(content string) (string, string, string, error) {
	content = strings.TrimPrefix(content, bodyV2+"\n# ")
	title, rest, ok := strings.Cut(content, "\n\n## Notes\n\n")
	if !ok {
		return "", "", "", fmt.Errorf("%w: missing notes section", domain.ErrInvalidInput)
	}
	notes, rest, err := takePayload(rest, "notes")
	return title, notes, rest, err
}

func parseWaitingBody(content string) (waitingBody, error) {
	if strings.HasPrefix(strings.TrimSpace(content), bodyV2+"\n") {
		title, notes, rest, err := parseV2Notes(strings.TrimLeft(content, "\r\n"))
		if err != nil {
			return waitingBody{}, err
		}
		rest = strings.TrimLeft(rest, "\n")
		if !strings.HasPrefix(rest, "## Check history\n") {
			return waitingBody{}, fmt.Errorf("%w: missing history section", domain.ErrInvalidInput)
		}
		rest = strings.TrimPrefix(rest, "## Check history\n")
		body := waitingBody{Title: title, Notes: notes}
		for strings.TrimSpace(rest) != "" {
			rest = strings.TrimLeft(rest, "\n")
			if !strings.HasPrefix(rest, "<!-- waiting-mcp:check -->\n### ") {
				return waitingBody{}, fmt.Errorf("%w: invalid check block", domain.ErrInvalidInput)
			}
			rest = strings.TrimPrefix(rest, "<!-- waiting-mcp:check -->\n### ")
			header, tail, ok := strings.Cut(rest, "\n\n")
			if !ok {
				return waitingBody{}, fmt.Errorf("%w: incomplete check header", domain.ErrInvalidInput)
			}
			record, err := parseCheckHeader(header)
			if err != nil {
				return waitingBody{}, err
			}
			record.Message, rest, err = takePayload(tail, "message")
			if err != nil {
				return waitingBody{}, err
			}
			body.History = append(body.History, record)
		}
		return body, nil
	}
	sections := splitSections(content)
	title, ok := sections["#"]
	if !ok || strings.TrimSpace(title) == "" {
		return waitingBody{}, fmt.Errorf("%w: missing title section", domain.ErrInvalidInput)
	}
	history, err := parseCheckHistory(sections["## Check history"])
	if err != nil {
		return waitingBody{}, err
	}
	return waitingBody{Title: strings.TrimSpace(title), Notes: strings.TrimSpace(sections["## Notes"]), History: history}, nil
}

func parsePersonBody(content string) (personBody, error) {
	if strings.HasPrefix(strings.TrimSpace(content), bodyV2+"\n") {
		title, notes, rest, err := parseV2Notes(strings.TrimLeft(content, "\r\n"))
		if err != nil {
			return personBody{}, err
		}
		if strings.TrimSpace(rest) != "" {
			return personBody{}, fmt.Errorf("%w: unexpected person content", domain.ErrInvalidInput)
		}
		return personBody{Title: title, Notes: notes}, nil
	}
	sections := splitSections(content)
	title, ok := sections["#"]
	if !ok || strings.TrimSpace(title) == "" {
		return personBody{}, fmt.Errorf("%w: missing title section", domain.ErrInvalidInput)
	}
	return personBody{Title: strings.TrimSpace(title), Notes: strings.TrimSpace(sections["## Notes"])}, nil
}

func parseCheckHeader(header string) (domain.CheckRecord, error) {
	parts := strings.SplitN(header, " — ", 2)
	if len(parts) != 2 {
		return domain.CheckRecord{}, fmt.Errorf("%w: invalid history header %q", domain.ErrInvalidInput, header)
	}
	date, err := parseDate(parts[0])
	if err != nil {
		return domain.CheckRecord{}, err
	}
	record := domain.CheckRecord{Date: date, Action: parts[1]}
	if err := domain.ValidateCheck(record); err != nil {
		return domain.CheckRecord{}, err
	}
	return record, nil
}

// fence tracks CommonMark backtick/tilde fenced blocks in legacy Markdown.
func fence(line string, marker *byte, size *int) bool {
	trimmed := strings.TrimLeft(line, " ")
	indent := len(line) - len(trimmed)
	if indent > 3 || len(trimmed) < 3 {
		return *marker != 0
	}
	ch := trimmed[0]
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	if *marker != 0 {
		if ch == *marker && count >= *size && strings.TrimSpace(trimmed[count:]) == "" {
			*marker = 0
		}
		return true
	}
	if (ch == '`' || ch == '~') && count >= 3 {
		*marker = ch
		*size = count
		return true
	}
	return false
}

func splitSections(content string) map[string]string {
	lines := strings.Split(content, "\n")
	sections := make(map[string]string)
	var currentKey string
	var builder strings.Builder

	flush := func() {
		if currentKey == "" {
			return
		}
		sections[currentKey] = strings.TrimSpace(builder.String())
		builder.Reset()
	}

	var marker byte
	var size int
	for _, line := range lines {
		inFence := fence(line, &marker, &size)
		if !inFence && (line == "## Notes" || line == "## Check history") {
			flush()
			currentKey = strings.TrimSpace(line)
			continue
		}
		if !inFence && currentKey == "" && strings.HasPrefix(line, "# ") {
			flush()
			currentKey = "#"
			builder.WriteString(strings.TrimPrefix(line, "# "))
			builder.WriteString("\n")
			continue
		}
		if currentKey != "" {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	flush()
	return sections
}

func parseCheckHistory(content string) ([]domain.CheckRecord, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}

	lines := strings.Split(content, "\n")
	var records []domain.CheckRecord
	var current *domain.CheckRecord
	var message strings.Builder

	flush := func() {
		if current == nil {
			return
		}
		current.Message = strings.TrimSpace(message.String())
		records = append(records, *current)
		current = nil
		message.Reset()
	}

	var marker byte
	var size int
	for _, line := range lines {
		if !fence(line, &marker, &size) && strings.HasPrefix(line, "### ") {
			record, err := parseCheckHeader(strings.TrimPrefix(line, "### "))
			if err == nil {
				flush()
				current = &record
				continue
			}
			if current == nil {
				return nil, err
			}
		}
		if current != nil {
			message.WriteString(line)
			message.WriteString("\n")
		}
	}
	flush()
	return records, nil
}
