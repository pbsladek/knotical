package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

type Request struct {
	PromptText      string
	Model           string
	System          string
	Fragments       []string
	Shell           bool
	DescribeShell   bool
	Code            bool
	NoMD            bool
	Chat            string
	Repl            string
	Role            string
	Template        string
	Temperature     float64
	Schema          string
	TopP            float64
	Cache           bool
	Interaction     bool
	ContinueLast    bool
	NoStream        bool
	Extract         bool
	Save            string
	Log             bool
	NoLog           bool
	ExecuteMode     shell.ExecutionMode
	ForceRiskyShell bool
	SandboxRuntime  string
	SandboxImage    string
	SandboxNetwork  bool
	SandboxWrite    bool
}

type ChatSessionStore interface {
	LoadOrCreate(name string) (model.ChatSession, error)
	Save(session model.ChatSession) error
}

type FragmentLoader interface {
	Load(name string) (store.Fragment, error)
}

type RoleLoader interface {
	Load(name string) (store.Role, error)
}

type TemplateLoaderSaver interface {
	Load(name string) (store.Template, error)
	Save(template store.Template) error
}

type AliasLoader interface {
	Load() (map[string]string, error)
}

type Cache interface {
	Get(model string, system string, messages []model.Message, schema map[string]any, temperature *float64, topP *float64) (string, bool, error)
	Set(model string, system string, messages []model.Message, schema map[string]any, temperature *float64, topP *float64, response string) error
}

type Logs interface {
	Insert(entry model.LogEntry) error
}

type Dependencies struct {
	LoadConfig    func() (config.Config, error)
	ResolveAPIKey func(providerName string) (string, error)
	BuildProvider func(name string, apiKey string, apiBaseURL string, timeout time.Duration) (provider.Provider, error)
	ChatStore     ChatSessionStore
	FragmentStore FragmentLoader
	RoleStore     RoleLoader
	TemplateStore TemplateLoaderSaver
	AliasStore    AliasLoader
	CacheStore    Cache
	NewLogStore   func() Logs
	Printer       *output.Printer
	PromptAction  func(options shell.PromptOptions) (shell.Action, error)
	ConfirmShell  func(mode shell.ExecutionMode, report shell.RiskReport) (bool, error)
	ExecuteShell  func(req shell.ExecutionRequest) error
	ReadLastChat  func() (string, error)
	WriteLastChat func(name string) error
	Now           func() time.Time
	Stdin         io.Reader
}

type Service struct {
	deps Dependencies
}

type promptRunContext struct {
	cfg            config.Config
	promptText     string
	modelID        string
	systemPrompt   string
	renderMarkdown bool
	schemaValue    map[string]any
	providerName   string
	prov           provider.Provider
	chatName       string
	session        model.ChatSession
	messages       []model.Message
	tempPtr        *float64
	topPPtr        *float64
}

type replRunContext struct {
	cfg            config.Config
	modelID        string
	systemPrompt   string
	renderMarkdown bool
	providerName   string
	prov           provider.Provider
	session        model.ChatSession
	tempPtr        *float64
	topPPtr        *float64
}

func New(deps Dependencies) *Service {
	return &Service{deps: deps}
}

func Default(printer *output.Printer, stdin io.Reader) *Service {
	if printer == nil {
		printer = output.NewPrinter(os.Stdout)
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	logStore := store.NewLogStore(config.LogsDBPath())
	return New(Dependencies{
		LoadConfig:    config.Load,
		ResolveAPIKey: defaultResolveAPIKey,
		BuildProvider: provider.Build,
		ChatStore:     store.ChatStore{Dir: config.ChatCacheDir()},
		FragmentStore: store.FragmentStore{Dir: config.FragmentsDir()},
		RoleStore:     store.RoleStore{Dir: config.RolesDir()},
		TemplateStore: store.TemplateStore{Dir: config.TemplatesDir()},
		AliasStore:    store.JSONMapStore{Path: config.AliasesFilePath()},
		CacheStore:    store.CacheStore{Dir: config.CacheDir()},
		NewLogStore: func() Logs {
			return logStore
		},
		Printer:      printer,
		PromptAction: shell.PromptAction,
		ConfirmShell: shell.ConfirmRiskyExecution,
		ExecuteShell: shell.ExecuteCommand,
		ReadLastChat: func() (string, error) {
			payload, err := os.ReadFile(config.LastSessionPath())
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(payload)), nil
		},
		WriteLastChat: func(name string) error {
			if err := os.MkdirAll(filepath.Dir(config.LastSessionPath()), 0o700); err != nil {
				return err
			}
			return os.WriteFile(config.LastSessionPath(), []byte(name), 0o600)
		},
		Now:   func() time.Time { return time.Now().UTC() },
		Stdin: stdin,
	})
}
