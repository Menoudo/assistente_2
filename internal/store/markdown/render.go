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
		Kind:             kindWaiting,
		ID:               waiting.ID,
		Status:           string(waiting.Status),
		Responsible:      waiting.Responsible,
		ExpectedResult:   waiting.ExpectedResult,
		NextCheck:        formatDate(waiting.NextCheck),
		Context:          string(waiting.Context),
		Tags:             waiting.Tags,
		Blocker:          waiting.Blocker,
		CreatedAt:        formatTimestamp(waiting.CreatedAt),
		UpdatedAt:        formatTimestamp(waiting.UpdatedAt),
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

func renderWaitingBody(body waitingBody) string {
	var buf strings.Builder
	buf.WriteString("# ")
	buf.WriteString(body.Title)
	buf.WriteString("\n\n## Notes\n\n")
	buf.WriteString(strings.TrimSpace(body.Notes))
	buf.WriteString("\n\n## Check history\n\n")
	for _, record := range body.History {
		buf.WriteString("### ")
		buf.WriteString(formatDate(record.Date))
		buf.WriteString(" — ")
		buf.WriteString(record.Action)
		buf.WriteString("\n\n")
		buf.WriteString(strings.TrimSpace(record.Message))
		buf.WriteString("\n\n")
	}
	return buf.String()
}

func renderPersonBody(body personBody) string {
	var buf strings.Builder
	buf.WriteString("# ")
	buf.WriteString(body.Title)
	buf.WriteString("\n\n## Notes\n\n")
	buf.WriteString(strings.TrimSpace(body.Notes))
	buf.WriteString("\n")
	return buf.String()
}

func parseWaitingBody(content string) (waitingBody, error) {
	sections := splitSections(content)
	title, ok := sections["#"]
	if !ok || strings.TrimSpace(title) == "" {
		return waitingBody{}, fmt.Errorf("%w: missing title section", domain.ErrInvalidInput)
	}
	notes := strings.TrimSpace(sections["## Notes"])
	historyText := sections["## Check history"]
	history, err := parseCheckHistory(historyText)
	if err != nil {
		return waitingBody{}, err
	}
	return waitingBody{
		Title:   strings.TrimSpace(title),
		Notes:   notes,
		History: history,
	}, nil
}

func parsePersonBody(content string) (personBody, error) {
	sections := splitSections(content)
	title, ok := sections["#"]
	if !ok || strings.TrimSpace(title) == "" {
		return personBody{}, fmt.Errorf("%w: missing title section", domain.ErrInvalidInput)
	}
	return personBody{
		Title: strings.TrimSpace(title),
		Notes: strings.TrimSpace(sections["## Notes"]),
	}, nil
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

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			currentKey = strings.TrimSpace(line)
			continue
		}
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
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

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			flush()
			header := strings.TrimPrefix(line, "### ")
			parts := strings.SplitN(header, " — ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("%w: invalid history header %q", domain.ErrInvalidInput, header)
			}
			date, err := parseDate(parts[0])
			if err != nil {
				return nil, err
			}
			current = &domain.CheckRecord{
				Date:   date,
				Action: strings.TrimSpace(parts[1]),
			}
			continue
		}
		if current != nil {
			message.WriteString(line)
			message.WriteString("\n")
		}
	}
	flush()
	return records, nil
}
