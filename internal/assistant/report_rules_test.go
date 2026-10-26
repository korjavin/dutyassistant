package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRulesTools(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	d := deps{store: st}
	run := func(name, input string) string {
		t.Helper()
		tool, ok := findTool(adminTools, name)
		require.True(t, ok, name)
		require.False(t, tool.mutating(), "%s must apply without confirmation", name)
		out, err := tool.runNow(ctx, d, []byte(input))
		require.NoError(t, err, name)
		return out
	}
	rules := func() string {
		got, err := st.GetSetting(ctx, store.SettingReportRules)
		require.NoError(t, err)
		return got
	}

	out := run("get_report_rules", "")
	assert.Contains(t, out, llm.DailyReportGuidance())
	assert.Contains(t, out, "(none yet")
	assert.Contains(t, run("undo_report_rules", ""), "Nothing to undo")

	out = run("update_report_rules", `{"rules":" 1. Будь добрее, если не успели почистить зубы. "}`)
	assert.Contains(t, out, "Saved")
	assert.Equal(t, "1. Будь добрее, если не успели почистить зубы.", rules())
	assert.Contains(t, run("get_report_rules", ""), "House rules (editable):\n1. Будь добрее")
	assert.Contains(t, run("update_report_rules", `{"rules":"1. Будь добрее, если не успели почистить зубы."}`), "No change")

	run("update_report_rules", `{"rules":"1. Будь добрее, если не успели почистить зубы.\n2. В группе пиши короче."}`)
	assert.Contains(t, rules(), "2. В группе")

	// Undo restores the version before the last change; undo again re-applies it.
	assert.Contains(t, run("undo_report_rules", ""), "Restored")
	assert.Equal(t, "1. Будь добрее, если не успели почистить зубы.", rules())
	run("undo_report_rules", "")
	assert.Contains(t, rules(), "2. В группе")

	// Empty rules remove them all, and that is undoable too.
	assert.Contains(t, run("update_report_rules", `{"rules":""}`), "removed")
	assert.Empty(t, rules())
	run("undo_report_rules", "")
	assert.Contains(t, rules(), "2. В группе")

	tool, _ := findTool(adminTools, "update_report_rules")
	_, err = tool.runNow(ctx, d, []byte(`{"rules":"`+strings.Repeat("я", llm.MaxReportRulesChars+1)+`"}`))
	assert.ErrorContains(t, err, "too long")
	assert.Contains(t, rules(), "2. В группе")

	// Participants can neither read nor change the rules.
	for _, name := range []string{"get_report_rules", "update_report_rules", "undo_report_rules"} {
		_, ok := findTool(participantTools, name)
		assert.False(t, ok, name)
	}
}
