package mocks

import (
	"context"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/stretchr/testify/mock"
)

// MockLLMClient is a mock implementation of the LLM client for testing.
type MockLLMClient struct {
	mock.Mock
}

// TranslateToEnglish mocks the TranslateToEnglish method.
func (m *MockLLMClient) TranslateToEnglish(ctx context.Context, text string) (string, error) {
	args := m.Called(ctx, text)
	return args.String(0), args.Error(1)
}

// RefineMessage mocks the RefineMessage method.
func (m *MockLLMClient) RefineMessage(ctx context.Context, intent, vanilla string) string {
	args := m.Called(ctx, intent, vanilla)
	if args.Get(0) == nil {
		return vanilla
	}
	return args.String(0)
}

// ChoreNudge mocks the ChoreNudge method.
func (m *MockLLMClient) ChoreNudge(ctx context.Context, userName, choresContext string) (string, error) {
	args := m.Called(ctx, userName, choresContext)
	return args.String(0), args.Error(1)
}

// Ensure MockLLMClient implements the llm.ClientInterface interface
var _ llm.ClientInterface = (*MockLLMClient)(nil)
