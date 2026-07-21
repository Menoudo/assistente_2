package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priahin-i/assistente_2/internal/domain"
	"github.com/priahin-i/assistente_2/internal/store"
)

type WaitingTools struct {
	repo store.Repository
}

func NewWaitingTools(repo store.Repository) *WaitingTools {
	return &WaitingTools{repo: repo}
}

type WaitingCreateInput struct {
	ID               string   `json:"id" jsonschema:"unique slug for the waiting item"`
	Responsible      string   `json:"responsible" jsonschema:"person id from people/"`
	ExpectedResult   string   `json:"expected_result" jsonschema:"what should appear"`
	NextCheck        string   `json:"next_check" jsonschema:"next review date YYYY-MM-DD"`
	Context          string   `json:"context" jsonschema:"work or personal"`
	Notes            string   `json:"notes,omitempty" jsonschema:"optional delegation context"`
	ResultDeadline   string   `json:"result_deadline,omitempty" jsonschema:"optional result deadline YYYY-MM-DD"`
	PromisedDeadline string   `json:"promised_deadline,omitempty" jsonschema:"optional promised deadline YYYY-MM-DD"`
	Tags             []string `json:"tags,omitempty" jsonschema:"optional tags"`
	Blocker          string   `json:"blocker,omitempty" jsonschema:"optional blocker"`
}

type WaitingIDInput struct {
	ID string `json:"id" jsonschema:"waiting item id"`
}

type WaitingListInput struct {
	Status      string `json:"status,omitempty" jsonschema:"active, done, or cancelled"`
	Responsible string `json:"responsible,omitempty" jsonschema:"filter by person id"`
	Context     string `json:"context,omitempty" jsonschema:"work or personal"`
	DueBefore   string `json:"due_before,omitempty" jsonschema:"filter next_check on or before date YYYY-MM-DD"`
}

type WaitingUpdateInput struct {
	ID               string   `json:"id" jsonschema:"waiting item id"`
	Responsible      string   `json:"responsible,omitempty"`
	ExpectedResult   string   `json:"expected_result,omitempty"`
	NextCheck        string   `json:"next_check,omitempty"`
	Context          string   `json:"context,omitempty"`
	Notes            string   `json:"notes,omitempty"`
	ResultDeadline   string   `json:"result_deadline,omitempty"`
	PromisedDeadline string   `json:"promised_deadline,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Blocker          string   `json:"blocker,omitempty"`
	ClearBlocker     bool     `json:"clear_blocker,omitempty"`
}

type WaitingRecordCheckInput struct {
	ID               string `json:"id" jsonschema:"waiting item id"`
	Action           string `json:"action" jsonschema:"check action such as pinged, checked_gitlab, discussed"`
	Message          string `json:"message,omitempty" jsonschema:"optional details"`
	NextCheck        string `json:"next_check" jsonschema:"next review date YYYY-MM-DD"`
	Blocker          string `json:"blocker,omitempty"`
	ClearBlocker     bool   `json:"clear_blocker,omitempty"`
	PromisedDeadline string `json:"promised_deadline,omitempty"`
}

type WaitingStatusInput struct {
	ID      string `json:"id" jsonschema:"waiting item id"`
	Message string `json:"message,omitempty" jsonschema:"optional final note"`
}

type WaitingReviewDueInput struct {
	AsOf string `json:"as_of,omitempty" jsonschema:"review date YYYY-MM-DD, defaults to today UTC"`
}

type WaitingResponse struct {
	Waiting WaitingView `json:"waiting"`
}

type WaitingListResponse struct {
	Items []WaitingView `json:"items"`
	Count int           `json:"count"`
}

type WaitingReviewDueResponse struct {
	AsOf  string          `json:"as_of"`
	Items []DueWaitingView `json:"items"`
	Count int             `json:"count"`
}

type WaitingView struct {
	ID               string   `json:"id"`
	Status           string   `json:"status"`
	Responsible      string   `json:"responsible"`
	ExpectedResult   string   `json:"expected_result"`
	ResultDeadline   string   `json:"result_deadline,omitempty"`
	PromisedDeadline string   `json:"promised_deadline,omitempty"`
	NextCheck        string   `json:"next_check"`
	Context          string   `json:"context"`
	Tags             []string `json:"tags,omitempty"`
	Blocker          string   `json:"blocker,omitempty"`
	Notes            string   `json:"notes,omitempty"`
	CheckHistory     []CheckRecordView `json:"check_history,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type CheckRecordView struct {
	Date    string `json:"date"`
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
}

type DueWaitingView struct {
	ID             string `json:"id"`
	Responsible    string `json:"responsible"`
	ExpectedResult string `json:"expected_result"`
	NextCheck      string `json:"next_check"`
	Blocker        string `json:"blocker,omitempty"`
	DaysOverdue    int    `json:"days_overdue"`
}

func (t *WaitingTools) Create(ctx context.Context, req *mcp.CallToolRequest, input WaitingCreateInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	now := time.Now().UTC()
	nextCheck, err := parseDate(input.NextCheck)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	resultDeadline, err := optionalDate(input.ResultDeadline)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	promisedDeadline, err := optionalDate(input.PromisedDeadline)
	if err != nil {
		return nil, WaitingResponse{}, err
	}

	waiting := domain.Waiting{
		ID:               input.ID,
		Status:           domain.StatusActive,
		Responsible:      input.Responsible,
		ExpectedResult:   input.ExpectedResult,
		ResultDeadline:   resultDeadline,
		PromisedDeadline: promisedDeadline,
		NextCheck:        nextCheck,
		Context:          domain.Context(input.Context),
		Tags:             input.Tags,
		Blocker:          input.Blocker,
		Notes:            input.Notes,
		CheckHistory: []domain.CheckRecord{
			{
				Date:    now,
				Action:  "created",
				Message: "Waiting item created.",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if !isContext(waiting.Context) {
		return nil, WaitingResponse{}, fmt.Errorf("%w: context must be work or personal", domain.ErrInvalidInput)
	}
	if err := t.repo.CreateWaiting(ctx, waiting); err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) Get(ctx context.Context, req *mcp.CallToolRequest, input WaitingIDInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	waiting, err := t.repo.GetWaiting(ctx, input.ID)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) List(ctx context.Context, req *mcp.CallToolRequest, input WaitingListInput) (*mcp.CallToolResult, WaitingListResponse, error) {
	_ = req
	filter, err := toWaitingFilter(input)
	if err != nil {
		return nil, WaitingListResponse{}, err
	}
	items, err := t.repo.ListWaiting(ctx, filter)
	if err != nil {
		return nil, WaitingListResponse{}, err
	}
	views := make([]WaitingView, 0, len(items))
	for _, item := range items {
		views = append(views, toWaitingView(item))
	}
	return nil, WaitingListResponse{Items: views, Count: len(views)}, nil
}

func (t *WaitingTools) Update(ctx context.Context, req *mcp.CallToolRequest, input WaitingUpdateInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	waiting, err := t.repo.GetWaiting(ctx, input.ID)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	if input.Responsible != "" {
		waiting.Responsible = input.Responsible
	}
	if input.ExpectedResult != "" {
		waiting.ExpectedResult = input.ExpectedResult
	}
	if input.NextCheck != "" {
		nextCheck, err := parseDate(input.NextCheck)
		if err != nil {
			return nil, WaitingResponse{}, err
		}
		waiting.NextCheck = nextCheck
	}
	if input.Context != "" {
		contextValue := domain.Context(input.Context)
		if !isContext(contextValue) {
			return nil, WaitingResponse{}, fmt.Errorf("%w: context must be work or personal", domain.ErrInvalidInput)
		}
		waiting.Context = contextValue
	}
	if input.Notes != "" {
		waiting.Notes = input.Notes
	}
	if input.ResultDeadline != "" {
		resultDeadline, err := parseDate(input.ResultDeadline)
		if err != nil {
			return nil, WaitingResponse{}, err
		}
		waiting.ResultDeadline = &resultDeadline
	}
	if input.PromisedDeadline != "" {
		promisedDeadline, err := parseDate(input.PromisedDeadline)
		if err != nil {
			return nil, WaitingResponse{}, err
		}
		waiting.PromisedDeadline = &promisedDeadline
	}
	if input.Tags != nil {
		waiting.Tags = input.Tags
	}
	if input.ClearBlocker {
		waiting.Blocker = ""
	} else if input.Blocker != "" {
		waiting.Blocker = input.Blocker
	}
	if err := t.repo.UpdateWaiting(ctx, waiting); err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) RecordCheck(ctx context.Context, req *mcp.CallToolRequest, input WaitingRecordCheckInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	nextCheck, err := parseDate(input.NextCheck)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	var promisedDeadline *time.Time
	if input.PromisedDeadline != "" {
		parsed, err := parseDate(input.PromisedDeadline)
		if err != nil {
			return nil, WaitingResponse{}, err
		}
		promisedDeadline = &parsed
	}
	var blocker *string
	if input.ClearBlocker {
		empty := ""
		blocker = &empty
	} else if input.Blocker != "" {
		blocker = &input.Blocker
	}
	record := domain.CheckRecord{
		Date:    time.Now().UTC(),
		Action:  input.Action,
		Message: input.Message,
	}
	if err := t.repo.RecordCheck(ctx, input.ID, record, nextCheck, blocker, promisedDeadline); err != nil {
		return nil, WaitingResponse{}, err
	}
	waiting, err := t.repo.GetWaiting(ctx, input.ID)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) Complete(ctx context.Context, req *mcp.CallToolRequest, input WaitingStatusInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	record := &domain.CheckRecord{
		Date:    time.Now().UTC(),
		Action:  "completed",
		Message: input.Message,
	}
	if record.Message == "" {
		record.Message = "Marked as done."
	}
	if err := t.repo.SetWaitingStatus(ctx, input.ID, domain.StatusDone, record); err != nil {
		return nil, WaitingResponse{}, err
	}
	waiting, err := t.repo.GetWaiting(ctx, input.ID)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) Cancel(ctx context.Context, req *mcp.CallToolRequest, input WaitingStatusInput) (*mcp.CallToolResult, WaitingResponse, error) {
	_ = req
	record := &domain.CheckRecord{
		Date:    time.Now().UTC(),
		Action:  "cancelled",
		Message: input.Message,
	}
	if record.Message == "" {
		record.Message = "Marked as cancelled."
	}
	if err := t.repo.SetWaitingStatus(ctx, input.ID, domain.StatusCancelled, record); err != nil {
		return nil, WaitingResponse{}, err
	}
	waiting, err := t.repo.GetWaiting(ctx, input.ID)
	if err != nil {
		return nil, WaitingResponse{}, err
	}
	return nil, WaitingResponse{Waiting: toWaitingView(waiting)}, nil
}

func (t *WaitingTools) ReviewDue(ctx context.Context, req *mcp.CallToolRequest, input WaitingReviewDueInput) (*mcp.CallToolResult, WaitingReviewDueResponse, error) {
	_ = req
	asOf := time.Now().UTC()
	if input.AsOf != "" {
		parsed, err := parseDate(input.AsOf)
		if err != nil {
			return nil, WaitingReviewDueResponse{}, err
		}
		asOf = parsed
	}
	items, err := t.repo.ListWaiting(ctx, domain.WaitingFilter{
		Status:        ptrStatus(domain.StatusActive),
		DueOnOrBefore: &asOf,
	})
	if err != nil {
		return nil, WaitingReviewDueResponse{}, err
	}
	views := make([]DueWaitingView, 0, len(items))
	for _, item := range items {
		views = append(views, DueWaitingView{
			ID:             item.ID,
			Responsible:    item.Responsible,
			ExpectedResult: item.ExpectedResult,
			NextCheck:      formatDate(item.NextCheck),
			Blocker:        item.Blocker,
			DaysOverdue:    daysOverdue(item.NextCheck, asOf),
		})
	}
	return nil, WaitingReviewDueResponse{
		AsOf:  formatDate(asOf),
		Items: views,
		Count: len(views),
	}, nil
}

func toWaitingFilter(input WaitingListInput) (domain.WaitingFilter, error) {
	filter := domain.WaitingFilter{
		Responsible: input.Responsible,
	}
	if input.Status != "" {
		status := domain.Status(input.Status)
		if !isStatus(status) {
			return domain.WaitingFilter{}, fmt.Errorf("%w: invalid status", domain.ErrInvalidInput)
		}
		filter.Status = &status
	}
	if input.Context != "" {
		contextValue := domain.Context(input.Context)
		if !isContext(contextValue) {
			return domain.WaitingFilter{}, fmt.Errorf("%w: invalid context", domain.ErrInvalidInput)
		}
		filter.Context = &contextValue
	}
	if input.DueBefore != "" {
		dueBefore, err := parseDate(input.DueBefore)
		if err != nil {
			return domain.WaitingFilter{}, err
		}
		filter.DueOnOrBefore = &dueBefore
	}
	return filter, nil
}

func toWaitingView(waiting domain.Waiting) WaitingView {
	view := WaitingView{
		ID:             waiting.ID,
		Status:         string(waiting.Status),
		Responsible:    waiting.Responsible,
		ExpectedResult: waiting.ExpectedResult,
		NextCheck:      formatDate(waiting.NextCheck),
		Context:        string(waiting.Context),
		Tags:           waiting.Tags,
		Blocker:        waiting.Blocker,
		Notes:          waiting.Notes,
		CreatedAt:      formatTimestamp(waiting.CreatedAt),
		UpdatedAt:      formatTimestamp(waiting.UpdatedAt),
	}
	if waiting.ResultDeadline != nil {
		view.ResultDeadline = formatDate(*waiting.ResultDeadline)
	}
	if waiting.PromisedDeadline != nil {
		view.PromisedDeadline = formatDate(*waiting.PromisedDeadline)
	}
	for _, record := range waiting.CheckHistory {
		view.CheckHistory = append(view.CheckHistory, CheckRecordView{
			Date:    formatDate(record.Date),
			Action:  record.Action,
			Message: record.Message,
		})
	}
	return view
}

func parseDate(value string) (time.Time, error) {
	value = trim(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("%w: empty date", domain.ErrInvalidInput)
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid date %q", domain.ErrInvalidInput, value)
	}
	return parsed.UTC(), nil
}

func optionalDate(value string) (*time.Time, error) {
	value = trim(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseDate(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func daysOverdue(nextCheck, asOf time.Time) int {
	next := dateOnly(nextCheck.UTC())
	current := dateOnly(asOf.UTC())
	if !current.After(next) {
		return 0
	}
	return int(current.Sub(next).Hours()/24) + 1
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func ptrStatus(status domain.Status) *domain.Status {
	return &status
}

func isContext(value domain.Context) bool {
	return value == domain.ContextWork || value == domain.ContextPersonal
}

func isStatus(value domain.Status) bool {
	return value == domain.StatusActive || value == domain.StatusDone || value == domain.StatusCancelled
}

func trim(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\n') {
		value = value[1:]
	}
	for len(value) > 0 {
		last := value[len(value)-1]
		if last != ' ' && last != '\t' && last != '\n' {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}

func ToolError(err error) (*mcp.CallToolResult, any, error) {
	if err == nil {
		return nil, nil, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	if errors.Is(err, domain.ErrAlreadyExists) || errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrResponsibleAbsent) || errors.Is(err, domain.ErrOverwriteForbidden) || errors.Is(err, domain.ErrNameNotUnique) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	return nil, nil, err
}
