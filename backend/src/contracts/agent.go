package contracts

type TradeSide string

const (
	TradeSideBuy  TradeSide = "buy"
	TradeSideSell TradeSide = "sell"
)

type TradeShape string

const (
	ShapeUnknown    TradeShape = ""
	ShapeScalp      TradeShape = "scalp"
	ShapeSwing      TradeShape = "swing"
	ShapeInvestment TradeShape = "investment"
)

type IntakeField struct {
	Key      string   `json:"key"`
	Question string   `json:"question"`
	Why      string   `json:"why,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// TradeIntent is what Executor's own LLM call decides this cycle — ticker,
// side, and amount are call-time outputs, never pre-locked on the Task.
// Amount is IDRX-equivalent, clamped server-side to the armed TradePermission's remaining headroom.
type TradeIntent struct {
	Ticker string    `json:"ticker"`
	Side   TradeSide `json:"side"`
	Amount string    `json:"amount"`
}

type CardPreset struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CardStep struct {
	N      string `json:"n"`
	Call   string `json:"call"`
	Detail string `json:"detail"`
}

type CardButtonLabels struct {
	Ready               string `json:"ready"`
	Arming              string `json:"arming"`
	Approving           string `json:"approving"`
	SubmittingApprove   string `json:"submitting_approve,omitempty"`
	Executing           string `json:"executing"`
	Executed            string `json:"executed"`
	ConnectWallet       string `json:"connect_wallet"`
	EnterBudget         string `json:"enter_budget"`
	AcknowledgeRequired string `json:"acknowledge_required"`
}

type CardFootnotes struct {
	SignaturesNeeded string `json:"signatures_needed"`
	ExecutedSuccess  string `json:"executed_success"`
	ExecutionFailed  string `json:"execution_failed"`
}

type CardNeedsInput struct {
	Title          string `json:"title,omitempty"`
	Notice         string `json:"notice"`
	Placeholder    string `json:"placeholder"`
	Button         string `json:"button"`
	SendingButton  string `json:"sending_button,omitempty"`
	CancelButton   string `json:"cancel_button,omitempty"`
	CancelledTitle string `json:"cancelled_title,omitempty"`
	CancelledDesc  string `json:"cancelled_desc,omitempty"`
}

type CardLedger struct {
	ArmedTitle       string `json:"armed_title"`
	ExecutedTitle    string `json:"executed_title"`
	PausedTitle      string `json:"paused_title"`
	PausedDesc       string `json:"paused_desc"`
	DisarmedTitle    string `json:"disarmed_title"`
	DisarmedDesc     string `json:"disarmed_desc"`
	ExecutedDesc     string `json:"executed_desc"`
	ToggleShow       string `json:"toggle_show"`
	ToggleHide       string `json:"toggle_hide"`
	TradesHeader     string `json:"trades_header"`
	NoTradesYet      string `json:"no_trades_yet"`
	MultiTradeNotice string `json:"multi_trade_notice"`
	DisarmNotice     string `json:"disarm_notice"`
	ResumeButton     string `json:"resume_button"`
	PauseButton      string `json:"pause_button"`
	DisarmButton     string `json:"disarm_button"`
}

type CardContract struct {
	TaskBadge          string            `json:"task_badge"`
	SubtaskUnit        string            `json:"subtask_unit"`
	HeaderDescription  string            `json:"header_description"`
	GenesisLabel       string            `json:"genesis_label"`
	ArmTitleReady      string            `json:"arm_title_ready"`
	ArmTitleArmed      string            `json:"arm_title_armed"`
	ArmDescription     string            `json:"arm_description"`
	NoTradeDescription string            `json:"no_trade_description"`
	BudgetLabel        string            `json:"budget_label"`
	BudgetPlaceholder  string            `json:"budget_placeholder"`
	PresetLabel        string            `json:"preset_label"`
	Presets            []CardPreset      `json:"presets"`
	Steps              []CardStep        `json:"steps"`
	Disclaimer         string            `json:"disclaimer"`
	ButtonLabels       CardButtonLabels  `json:"button_labels"`
	Footnotes          CardFootnotes     `json:"footnotes"`
	NeedsInput         CardNeedsInput    `json:"needs_input"`
	StatusLabels       map[string]string `json:"status_labels"`
	Ledger             CardLedger        `json:"ledger"`

	// Dynamic Guardrails & Shape-Aware UI (100% User-Driven Language)
	ShapeBadge             string            `json:"shape_badge,omitempty"`
	GuardrailLabel         string            `json:"guardrail_label,omitempty"`
	GuardrailHideLabel     string            `json:"guardrail_hide_label,omitempty"`
	RecurringLabel         string            `json:"recurring_label,omitempty"`
	MaxPerTradeLabel       string            `json:"max_per_trade_label,omitempty"`
	MaxPerTradePlaceholder string            `json:"max_per_trade_placeholder,omitempty"`
	CooldownLabel          string            `json:"cooldown_label,omitempty"`
	CooldownNoneOption     string            `json:"cooldown_none_option,omitempty"`
	Cooldown1hOption       string            `json:"cooldown_1h_option,omitempty"`
	Cooldown4hOption       string            `json:"cooldown_4h_option,omitempty"`
	Cooldown1dOption       string            `json:"cooldown_1d_option,omitempty"`
	Cooldown1wOption       string            `json:"cooldown_1w_option,omitempty"`
	HorizonLabel           string            `json:"horizon_label,omitempty"`
	HorizonNotice          string            `json:"horizon_notice,omitempty"`
	SellBudgetLabel        string            `json:"sell_budget_label,omitempty"`
	SellBudgetPlaceholder  string            `json:"sell_budget_placeholder,omitempty"`
	SellArmDescription     string            `json:"sell_arm_description,omitempty"`
	SellPresets            []CardPreset      `json:"sell_presets,omitempty"`
}

