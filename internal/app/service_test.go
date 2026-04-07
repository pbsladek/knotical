package app

import (
	"context"
	"errors"

	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

type fakeProvider struct {
	response  model.CompletionResponse
	responses []model.CompletionResponse
	requests  []provider.Request
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Complete(_ context.Context, req provider.Request) (model.CompletionResponse, error) {
	p.requests = append(p.requests, req)
	if len(p.responses) > 0 {
		resp := p.responses[0]
		p.responses = p.responses[1:]
		return resp, nil
	}
	return p.response, nil
}
func (p *fakeProvider) Stream(_ context.Context, req provider.Request, emit func(model.StreamChunk) error) error {
	p.requests = append(p.requests, req)
	resp := p.response
	if len(p.responses) > 0 {
		resp = p.responses[0]
		p.responses = p.responses[1:]
	}
	if resp.Content != "" {
		if err := emit(model.StreamChunk{Delta: resp.Content}); err != nil {
			return err
		}
	}
	if resp.Usage != nil {
		return emit(model.StreamChunk{Usage: resp.Usage, Done: true})
	}
	return nil
}
func (p *fakeProvider) ListModels(context.Context) ([]string, error) { return nil, nil }

type fakeChatStore struct {
	session model.ChatSession
	saved   []model.ChatSession
}

func (s *fakeChatStore) LoadOrCreate(name string) (model.ChatSession, error) {
	if s.session.Name == "" {
		s.session = model.NewChatSession(name)
	}
	return s.session, nil
}
func (s *fakeChatStore) Save(session model.ChatSession) error {
	s.session = session
	s.saved = append(s.saved, session)
	return nil
}

type fakeFragmentStore struct {
	fragments map[string]store.Fragment
}

func (s fakeFragmentStore) Load(name string) (store.Fragment, error) {
	fragment, ok := s.fragments[name]
	if !ok {
		return store.Fragment{}, errors.New("missing fragment")
	}
	return fragment, nil
}

type fakeRoleStore struct {
	role store.Role
}

func (s fakeRoleStore) Load(name string) (store.Role, error) {
	if s.role.Name == name {
		return s.role, nil
	}
	return store.Role{}, errors.New("missing role")
}

type fakeTemplateStore struct {
	templates map[string]store.Template
	saved     []store.Template
}

func (s *fakeTemplateStore) Load(name string) (store.Template, error) {
	template, ok := s.templates[name]
	if !ok {
		return store.Template{}, errors.New("missing template")
	}
	return template, nil
}
func (s *fakeTemplateStore) Save(template store.Template) error {
	s.saved = append(s.saved, template)
	return nil
}

type fakeAliasStore struct {
	aliases map[string]string
}

func (s fakeAliasStore) Load() (map[string]string, error) {
	return s.aliases, nil
}

type fakeCacheStore struct {
	value      string
	ok         bool
	sets       []string
	lastSchema map[string]any
}

func (s *fakeCacheStore) Get(_ string, _ string, _ []model.Message, schema map[string]any, _ *float64, _ *float64) (string, bool, error) {
	s.lastSchema = schema
	return s.value, s.ok, nil
}
func (s *fakeCacheStore) Set(_ string, _ string, _ []model.Message, schema map[string]any, _ *float64, _ *float64, response string) error {
	s.lastSchema = schema
	s.sets = append(s.sets, response)
	return nil
}

type fakeLogs struct {
	entries []model.LogEntry
}

func (l *fakeLogs) Insert(entry model.LogEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

type fakeShellExecutor struct {
	requests []shell.ExecutionRequest
}

func (e *fakeShellExecutor) Execute(req shell.ExecutionRequest) error {
	e.requests = append(e.requests, req)
	return nil
}

func serviceReq(configure func(*Request)) Request {
	var req Request
	if configure != nil {
		configure(&req)
	}
	return req
}
