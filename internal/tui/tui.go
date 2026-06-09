package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/orchestrator"
)

const appVersion = "1.0.0"

type screen int

const (
	screenHome screen = iota
	screenForm
	screenRunning
	screenResult
	screenError
	screenConfig
	screenVersion
)

type actionKind string

const (
	actionValidate actionKind = "validate"
	actionBackup   actionKind = "backup"
	actionRestore  actionKind = "restore"
	actionConfig   actionKind = "config"
	actionVersion  actionKind = "version"
)

type fieldKind int

const (
	fieldString fieldKind = iota
	fieldPassword
	fieldInt
	fieldEnum
)

type field struct {
	label    string
	kind     fieldKind
	required bool
	input    textinput.Model
	options  []string
	idx      int
}

func newStringField(label, value string, required bool, secret bool) field {
	ti := textinput.New()
	ti.SetValue(value)
	ti.CharLimit = 256
	ti.Width = 34
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle()
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}
	return field{label: label, kind: func() fieldKind {
		if secret {
			return fieldPassword
		}
		return fieldString
	}(), required: required, input: ti}
}

func newIntField(label string, value int, required bool) field {
	ti := textinput.New()
	if value != 0 {
		ti.SetValue(strconv.Itoa(value))
	}
	ti.CharLimit = 12
	ti.Width = 12
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle()
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	return field{label: label, kind: fieldInt, required: required, input: ti}
}

func newEnumField(label string, options []string, current string, required bool) field {
	idx := 0
	for i, option := range options {
		if strings.EqualFold(option, current) {
			idx = i
			break
		}
	}
	return field{label: label, kind: fieldEnum, required: required, options: options, idx: idx}
}

func (f *field) focus() {
	if f.kind == fieldEnum {
		return
	}
	f.input.Focus()
}

func (f *field) blur() {
	if f.kind == fieldEnum {
		return
	}
	f.input.Blur()
}

func (f *field) update(msg tea.Msg) tea.Cmd {
	if f.kind == fieldEnum {
		return nil
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	return cmd
}

func (f field) value() string {
	switch f.kind {
	case fieldEnum:
		if len(f.options) == 0 {
			return ""
		}
		return f.options[f.idx]
	default:
		return strings.TrimSpace(f.input.Value())
	}
}

func (f *field) setValue(v string) {
	switch f.kind {
	case fieldEnum:
		for i, option := range f.options {
			if strings.EqualFold(option, v) {
				f.idx = i
				return
			}
		}
	case fieldInt:
		f.input.SetValue(strings.TrimSpace(v))
	default:
		f.input.SetValue(v)
	}
}

func (f *field) cycle(delta int) {
	if f.kind != fieldEnum || len(f.options) == 0 {
		return
	}
	f.idx = (f.idx + delta + len(f.options)) % len(f.options)
}

type menuItem struct {
	label  string
	action actionKind
}

type runResultMsg struct {
	action  actionKind
	outcome any
	err     error
	code    int
}

type tickMsg time.Time

type model struct {
	cfgFile string
	baseCfg *config.Config
	cfg     config.Config

	screen   screen
	homeIdx  int
	formIdx  int
	homeMenu []menuItem
	fields   []field
	action   actionKind

	running   bool
	startedAt time.Time
	elapsed   time.Duration
	spinner   spinner.Model

	viewport   viewport.Model
	errText    string
	result     string
	resultCode int
}

func Run(cfg *config.Config, cfgFile string) error {
	m := newModel(cfg, cfgFile)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(cfg *config.Config, cfgFile string) model {
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	m := model{
		cfgFile: cfgFile,
		baseCfg: cfg,
		cfg:     *cfg,
		screen:  screenHome,
		homeMenu: []menuItem{
			{label: "Validate", action: actionValidate},
			{label: "Backup", action: actionBackup},
			{label: "Restore", action: actionRestore},
			{label: "Config", action: actionConfig},
			{label: "Version", action: actionVersion},
			{label: "Quit", action: ""},
		},
		spinner: sp,
	}
	m.viewport = viewport.New(0, 0)
	m.viewport.Style = lipgloss.NewStyle()
	m.viewport.SetContent("Loading...")
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickEverySecond())
}

func tickEverySecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		return m, nil
	case tickMsg:
		if m.running {
			m.elapsed = time.Since(m.startedAt)
			return m, tickEverySecond()
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case runResultMsg:
		m.running = false
		m.resultCode = msg.code
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.screen = screenError
			m.viewport.SetContent(renderErrorContent(msg.action, msg.err, msg.code))
			return m, nil
		}
		m.result = renderOutcome(msg.action, msg.outcome)
		m.screen = screenResult
		m.viewport.SetContent(m.result)
		return m, nil
	case tea.KeyMsg:
		switch m.screen {
		case screenHome:
			return m.updateHome(msg)
		case screenForm:
			return m.updateForm(msg)
		case screenRunning:
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				return m, tea.Quit
			}
			return m, nil
		case screenResult, screenError, screenConfig, screenVersion:
			switch msg.String() {
			case "ctrl+c", "q", "esc", "enter":
				m.screen = screenHome
				m.viewport.SetContent("")
				return m, nil
			}
			return m, nil
		}
	}

	if m.screen == screenConfig || m.screen == screenVersion {
		return m, nil
	}

	return m, nil
}

func (m model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.homeIdx > 0 {
			m.homeIdx--
		}
	case "down":
		if m.homeIdx < len(m.homeMenu)-1 {
			m.homeIdx++
		}
	case "enter":
		item := m.homeMenu[m.homeIdx]
		if item.action == "" {
			return m, tea.Quit
		}
		return m.startAction(item.action)
	case "ctrl+c", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) startAction(action actionKind) (tea.Model, tea.Cmd) {
	switch action {
	case actionConfig:
		m.screen = screenConfig
		m.viewport.SetContent(renderConfigContent(m.baseCfg))
		return m, nil
	case actionVersion:
		m.screen = screenVersion
		m.viewport.SetContent(fmt.Sprintf("backupdb version %s\n", appVersion))
		return m, nil
	case actionValidate, actionBackup, actionRestore:
		m.action = action
		m.fields = buildFields(m.baseCfg, action)
		m.formIdx = 0
		setFocus(m.fields, 0)
		m.screen = screenForm
		return m, nil
	default:
		return m, nil
	}
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.fields) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenHome
		return m, nil
	case "down":
		setFocus(m.fields, (m.formIdx+1)%len(m.fields))
		m.formIdx = (m.formIdx + 1) % len(m.fields)
		return m, nil
	case "up":
		m.formIdx = (m.formIdx - 1 + len(m.fields)) % len(m.fields)
		setFocus(m.fields, m.formIdx)
		return m, nil
	case "left":
		if m.fields[m.formIdx].kind == fieldEnum {
			m.fields[m.formIdx].cycle(-1)
			return m, nil
		}
	case "right":
		if m.fields[m.formIdx].kind == fieldEnum {
			m.fields[m.formIdx].cycle(1)
			return m, nil
		}
	case "enter":
		if m.formIdx < len(m.fields)-1 {
			m.formIdx++
			setFocus(m.fields, m.formIdx)
			return m, nil
		}
		return m.submit()
	}

	var cmd tea.Cmd
	m.fields[m.formIdx].input, cmd = m.fields[m.formIdx].input.Update(msg)
	return m, cmd
}

func (m model) submit() (tea.Model, tea.Cmd) {
	cfg := *m.baseCfg
	applyFields(&cfg, m.fields, m.action)

	if err := cfg.Validate(string(m.action)); err != nil {
		m.screen = screenError
		m.errText = err.Error()
		m.viewport.SetContent(renderErrorContent(m.action, err, 2))
		return m, nil
	}

	orch := orchestrator.NewOrchestrator(&cfg)
	m.running = true
	m.startedAt = time.Now()
	m.elapsed = 0
	m.screen = screenRunning
	m.viewport.SetContent("")

	return m, tea.Batch(m.spinner.Tick, tickEverySecond(), func() tea.Msg {
		switch m.action {
		case actionValidate:
			err, code := orch.RunValidate()
			return runResultMsg{action: m.action, err: err, code: code}
		case actionBackup:
			outcome, err, code := orch.RunBackup()
			return runResultMsg{action: m.action, outcome: outcome, err: err, code: code}
		case actionRestore:
			outcome, err, code := orch.RunRestore()
			return runResultMsg{action: m.action, outcome: outcome, err: err, code: code}
		default:
			return runResultMsg{action: m.action, err: fmt.Errorf("unsupported action"), code: 2}
		}
	})
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	panelStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	switch m.screen {
	case screenHome:
		var b strings.Builder
		b.WriteString(titleStyle.Render("BackupDB"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("Interactive terminal UI"))
		b.WriteString("\n\n")
		for i, item := range m.homeMenu {
			prefix := "  "
			if i == m.homeIdx {
				prefix = " >"
			}
			line := fmt.Sprintf("%s %s", prefix, item.label)
			if i == m.homeIdx {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(normalStyle.Render(line))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("Use arrow keys, Enter to select, q to quit"))
		return panelStyle.Render(b.String())
	case screenForm:
		var b strings.Builder
		b.WriteString(titleStyle.Render(strings.ToUpper(string(m.action))))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("Use arrow keys, Enter to submit, Esc to return"))
		b.WriteString("\n\n")
		for i, f := range m.fields {
			b.WriteString(renderField(f, i == m.formIdx))
			b.WriteString("\n")
		}
		if m.running {
			b.WriteString("\n")
			b.WriteString(m.spinner.View())
		}
		return panelStyle.Render(b.String())
	case screenRunning:
		return panelStyle.Render(fmt.Sprintf("%s\n\nRunning %s...\nElapsed: %s\n", m.spinner.View(), strings.ToUpper(string(m.action)), m.elapsed.Truncate(time.Second)))
	case screenResult:
		return panelStyle.Render("Result\n\n" + m.viewport.View())
	case screenError:
		return panelStyle.Render("Error\n\n" + m.viewport.View())
	case screenConfig, screenVersion:
		return panelStyle.Render(m.viewport.View())
	default:
		return panelStyle.Render("Unknown screen")
	}
}

func setFocus(fields []field, idx int) {
	for i := range fields {
		if i == idx {
			fields[i].focus()
		} else {
			fields[i].blur()
		}
	}
}

func renderField(f field, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("223"))
	placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	label := labelStyle.Render(fmt.Sprintf("%-18s", f.label))
	if focused {
		label = focusStyle.Render(fmt.Sprintf("%-18s", f.label))
	}

	var value string
	switch f.kind {
	case fieldEnum:
		value = strings.Join(f.options, " / ")
		if len(f.options) > 0 {
			value = fmt.Sprintf("%s  [%s]", strings.Join(f.options, " / "), f.options[f.idx])
		}
		value = valueStyle.Render(value)
	default:
		value = f.input.View()
		if strings.TrimSpace(f.input.Value()) == "" {
			value = placeholderStyle.Render("(empty)")
		}
	}

	if focused {
		return fmt.Sprintf("%s %s", label, value)
	}
	return fmt.Sprintf("%s %s", labelStyle.Render(f.label), value)
}

func buildFields(cfg *config.Config, action actionKind) []field {
	fields := []field{
		newEnumField("DB Type", []string{"sqlite", "postgres", "mysql", "mongodb"}, cfg.DB.Type, true),
		newStringField("Host", cfg.DB.Host, action != actionValidate || cfg.DB.Type != "sqlite", false),
		newIntField("Port", cfg.DB.Port, false),
		newStringField("User", cfg.DB.User, false, false),
		newStringField("Password", cfg.DB.Password, false, true),
		newStringField("Database", cfg.DB.Database, true, false),
	}

	switch action {
	case actionBackup:
		fields = append(fields,
			newEnumField("Backup Mode", []string{"full", "incremental", "differential"}, cfg.Backup.Mode, true),
			newEnumField("Compression", []string{"none", "gzip", "zstd"}, cfg.Backup.Compress, true),
			newEnumField("Storage", []string{"local", "s3", "gcs", "azure"}, cfg.Storage.Type, true),
			newStringField("Local Path", cfg.Storage.LocalPath, false, false),
			newStringField("Bucket", cfg.Storage.Bucket, false, false),
			newStringField("Prefix", cfg.Storage.Prefix, false, false),
			newStringField("Region", cfg.Storage.Region, false, false),
			newStringField("Endpoint", cfg.Storage.Endpoint, false, false),
			newStringField("Access Key", cfg.Storage.AccessKey, false, false),
			newStringField("Secret Key", cfg.Storage.SecretKey, false, true),
			newStringField("Container", cfg.Storage.Container, false, false),
			newStringField("Azure Name", cfg.Storage.AzureAccountName, false, false),
			newStringField("Azure Key", cfg.Storage.AzureAccountKey, false, true),
			newStringField("GCS Creds", cfg.Storage.GCSCredentialsFile, false, false),
		)
	case actionRestore:
		fields = append(fields,
			newStringField("Backup Path", cfg.Restore.BackupPath, true, false),
			newStringField("Tables", strings.Join(cfg.Restore.Tables, ","), false, false),
			newStringField("Collections", strings.Join(cfg.Restore.Collections, ","), false, false),
			newEnumField("Storage", []string{"local", "s3", "gcs", "azure"}, cfg.Storage.Type, true),
			newStringField("Local Path", cfg.Storage.LocalPath, false, false),
			newStringField("Bucket", cfg.Storage.Bucket, false, false),
			newStringField("Prefix", cfg.Storage.Prefix, false, false),
			newStringField("Region", cfg.Storage.Region, false, false),
			newStringField("Endpoint", cfg.Storage.Endpoint, false, false),
			newStringField("Access Key", cfg.Storage.AccessKey, false, false),
			newStringField("Secret Key", cfg.Storage.SecretKey, false, true),
			newStringField("Container", cfg.Storage.Container, false, false),
			newStringField("Azure Name", cfg.Storage.AzureAccountName, false, false),
			newStringField("Azure Key", cfg.Storage.AzureAccountKey, false, true),
			newStringField("GCS Creds", cfg.Storage.GCSCredentialsFile, false, false),
		)
	}

	return fields
}

func applyFields(cfg *config.Config, fields []field, action actionKind) {
	for _, f := range fields {
		switch f.label {
		case "DB Type":
			cfg.DB.Type = f.value()
		case "Host":
			cfg.DB.Host = f.value()
		case "Port":
			if v := f.value(); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					cfg.DB.Port = n
				}
			}
		case "User":
			cfg.DB.User = f.value()
		case "Password":
			cfg.DB.Password = f.value()
		case "Database":
			cfg.DB.Database = f.value()
		case "Backup Mode":
			cfg.Backup.Mode = f.value()
		case "Compression":
			cfg.Backup.Compress = f.value()
		case "Storage":
			cfg.Storage.Type = f.value()
		case "Local Path":
			cfg.Storage.LocalPath = f.value()
		case "Bucket":
			cfg.Storage.Bucket = f.value()
		case "Prefix":
			cfg.Storage.Prefix = f.value()
		case "Region":
			cfg.Storage.Region = f.value()
		case "Endpoint":
			cfg.Storage.Endpoint = f.value()
		case "Access Key":
			cfg.Storage.AccessKey = f.value()
		case "Secret Key":
			cfg.Storage.SecretKey = f.value()
		case "Container":
			cfg.Storage.Container = f.value()
		case "Azure Name":
			cfg.Storage.AzureAccountName = f.value()
		case "Azure Key":
			cfg.Storage.AzureAccountKey = f.value()
		case "GCS Creds":
			cfg.Storage.GCSCredentialsFile = f.value()
		case "Backup Path":
			cfg.Restore.BackupPath = f.value()
		case "Tables":
			if v := f.value(); v != "" {
				cfg.Restore.Tables = strings.Split(v, ",")
				for i := range cfg.Restore.Tables {
					cfg.Restore.Tables[i] = strings.TrimSpace(cfg.Restore.Tables[i])
				}
			} else {
				cfg.Restore.Tables = nil
			}
		case "Collections":
			if v := f.value(); v != "" {
				cfg.Restore.Collections = strings.Split(v, ",")
				for i := range cfg.Restore.Collections {
					cfg.Restore.Collections[i] = strings.TrimSpace(cfg.Restore.Collections[i])
				}
			} else {
				cfg.Restore.Collections = nil
			}
		}
	}
	if action == actionValidate {
		cfg.Storage.Type = ""
	}
}

func renderConfigContent(cfg *config.Config) string {
	redacted := cfg.Redact()
	out, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to render config: %v", err)
	}
	return string(out)
}

func renderOutcome(action actionKind, outcome any) string {
	switch v := outcome.(type) {
	case nil:
		return fmt.Sprintf("%s completed successfully.", strings.ToUpper(string(action)))
	case string:
		return v
	default:
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%+v", v)
		}
		return string(out)
	}
}

func renderErrorContent(action actionKind, err error, code int) string {
	return fmt.Sprintf("%s failed\n\nExit code: %d\nError: %s", strings.ToUpper(string(action)), code, err.Error())
}
