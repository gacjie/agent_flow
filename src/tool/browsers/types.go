package browsers

type SelectorState struct {
	Selector string `json:"selector"`
	Exists   bool   `json:"exists"`
	Visible  bool   `json:"visible"`
	Text     string `json:"text,omitempty"`
	Error    string `json:"error,omitempty"`
}

type FormField struct {
	Selector    string `json:"selector,omitempty"`
	Label       string `json:"label,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Filled      bool   `json:"filled,omitempty"`
}

type FormFrame struct {
	Selector    string      `json:"selector,omitempty"`
	Title       string      `json:"title,omitempty"`
	Method      string      `json:"method,omitempty"`
	Action      string      `json:"action,omitempty"`
	FieldCount  int         `json:"field_count,omitempty"`
	SubmitTexts []string    `json:"submit_texts,omitempty"`
	Fields      []FormField `json:"fields,omitempty"`
}

type PageFrame struct {
	MainHeading string      `json:"main_heading,omitempty"`
	LayoutHints []string    `json:"layout_hints,omitempty"`
	Forms       []FormFrame `json:"forms,omitempty"`
}

type StructuredState struct {
	Title              string           `json:"title"`
	URL                string           `json:"url"`
	Alerts             []string         `json:"alerts,omitempty"`
	Validation         []string         `json:"validation,omitempty"`
	KeyTexts           []string         `json:"key_texts,omitempty"`
	Watched            []SelectorState  `json:"watched,omitempty"`
	PageFrame          PageFrame        `json:"page_frame,omitempty"`
	StructuredSnapshot string           `json:"structured_snapshot,omitempty"`
	InteractionRefs    []InteractionRef `json:"interaction_refs,omitempty"`
	PageSource         string           `json:"page_source,omitempty"`
	ActiveElement      string           `json:"active_element,omitempty"`
}

type DocState struct {
	URL      string `json:"url,omitempty"`
	Status   int    `json:"status,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

type StepSignals struct {
	PreviousURL        string           `json:"previous_url,omitempty"`
	PreviousTitle      string           `json:"previous_title,omitempty"`
	URL                string           `json:"url,omitempty"`
	Title              string           `json:"title,omitempty"`
	NavigationChanged  bool             `json:"navigation_changed"`
	MainDocument       DocState         `json:"main_document"`
	ActiveElement      string           `json:"active_element,omitempty"`
	PageFrame          *PageFrame       `json:"page_frame,omitempty"`
	StructuredSnapshot string           `json:"structured_snapshot,omitempty"`
	InteractionRefs    []InteractionRef `json:"interaction_refs,omitempty"`
	PageSource         string           `json:"page_source,omitempty"`
	Alerts             []string         `json:"alerts,omitempty"`
	Validation         []string         `json:"validation,omitempty"`
	KeyTexts           []string         `json:"key_texts,omitempty"`
	Watched            []SelectorState  `json:"watched,omitempty"`
	ConsoleErrors      []string         `json:"console_errors,omitempty"`
	ConsoleWarnings    []string         `json:"console_warnings,omitempty"`
	LoadingFailures    []string         `json:"loading_failures,omitempty"`
	NetworkEvents      []string         `json:"network_events,omitempty"`
	EvalResult         interface{}      `json:"eval_result,omitempty"`
}

type StructuredStepResult struct {
	Index     int         `json:"index"`
	Action    string      `json:"action"`
	Selector  string      `json:"selector,omitempty"`
	OK        bool        `json:"ok"`
	ElapsedMS int64       `json:"elapsed_ms"`
	Summary   string      `json:"summary"`
	Signals   StepSignals `json:"signals"`
	Error     string      `json:"error,omitempty"`
}

type StructuredOutput struct {
	OK            bool                 `json:"ok"`
	StartedAt     string               `json:"started_at"`
	FinishedAt    string               `json:"finished_at"`
	OutputExpands []string             `json:"output_expands,omitempty"`
	Summary       string               `json:"summary"`
	Steps         []StructuredStepResult `json:"steps"`
	Error         string               `json:"error,omitempty"`
}

type OutputPlan struct {
	KeyTexts                int
	Watched                 int
	PageForms               int
	PageFields              int
	StructuredChars         int
	StructuredRefs          int
	StructuredDepth         int
	StructuredNodes         int
	ConsoleErrors           int
	ConsoleWarnings         int
	LoadingFailures         int
	NetworkEvents           int
	IncludePreviousContext  bool
	AlwaysIncludeStructured bool
	IncludePageSource       bool
}

type Marker struct {
	Console int
	Net     int
}

type Delta struct {
	Document        DocState
	ConsoleErrors   []string
	ConsoleWarnings []string
	LoadingFailures []string
	NetworkEvents   []string
}

type StructuredAction struct {
	Action    string
	URL       string
	Selector  string
	Text      string
	Script    string
	Direction string
	Amount    int
	Wait      int
}

type InteractionRef struct {
	Ref      string `json:"ref"`
	Selector string `json:"selector"`
	Role     string `json:"role,omitempty"`
	Name     string `json:"name,omitempty"`
}

type StepParams struct {
	Action    string `json:"action"`
	URL       string `json:"url"`
	Selector  string `json:"selector"`
	Text      string `json:"text"`
	Script    string `json:"script"`
	Direction string `json:"direction"`
	Amount    int    `json:"amount"`
	Wait      int    `json:"wait"`
}

type Params struct {
	Action           string       `json:"action"`
	Actions          []StepParams `json:"actions"`
	URL              string       `json:"url"`
	Selector         string       `json:"selector"`
	Text             string       `json:"text"`
	Script           string       `json:"script"`
	Direction        string       `json:"direction"`
	Amount           int          `json:"amount"`
	Wait             int          `json:"wait"`
	Width            int          `json:"width"`
	Height           int          `json:"height"`
	OutputExpands    []string     `json:"output_expands"`
	SnapshotSelector string       `json:"snapshot_selector"`
	WatchSelectors   []string     `json:"watch_selectors"`
	Debug            bool         `json:"debug"`
}
