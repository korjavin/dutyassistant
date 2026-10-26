package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store"
)

// The admin's house rules for daily report assessment: free text appended to
// the built-in prompt of llm.AssessDailyReport. The admin dictates amendments
// in free text and the assistant merges them into the stored rules, keeping
// the previous version for undo.

func readReportRules(ctx context.Context, d deps, _ []byte) (string, error) {
	rules, err := d.store.GetSetting(ctx, store.SettingReportRules)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rules) == "" {
		rules = "(none yet: only the built-in guidance applies)"
	}
	return "Built-in guidance (fixed):\n" + llm.DailyReportGuidance() + "\n\nHouse rules (editable):\n" + rules, nil
}

func applyReportRules(ctx context.Context, d deps, input []byte) (string, error) {
	var in struct {
		Rules string `json:"rules"`
	}
	if err := decode(input, &in); err != nil {
		return "", err
	}
	rules := strings.TrimSpace(in.Rules)
	if n := len([]rune(rules)); n > llm.MaxReportRulesChars {
		return "", fmt.Errorf("the rules are too long (%d characters, at most %d); make them shorter", n, llm.MaxReportRulesChars)
	}
	current, err := d.store.GetSetting(ctx, store.SettingReportRules)
	if err != nil {
		return "", err
	}
	if rules == current {
		return "No change: the house rules are already exactly this.", nil
	}
	if err := d.store.SetSetting(ctx, store.SettingReportRulesPrevious, current); err != nil {
		return "", err
	}
	if err := d.store.SetSetting(ctx, store.SettingReportRules, rules); err != nil {
		return "", err
	}
	if rules == "" {
		return "Saved: all house rules removed, only the built-in guidance applies.", nil
	}
	return "Saved. House rules now:\n" + rules, nil
}

func undoReportRules(ctx context.Context, d deps, _ []byte) (string, error) {
	current, err := d.store.GetSetting(ctx, store.SettingReportRules)
	if err != nil {
		return "", err
	}
	previous, err := d.store.GetSetting(ctx, store.SettingReportRulesPrevious)
	if err != nil {
		return "", err
	}
	if current == previous {
		return "Nothing to undo: the house rules were not changed.", nil
	}
	if err := d.store.SetSetting(ctx, store.SettingReportRules, previous); err != nil {
		return "", err
	}
	if err := d.store.SetSetting(ctx, store.SettingReportRulesPrevious, current); err != nil {
		return "", err
	}
	if previous == "" {
		return "Restored: no house rules, only the built-in guidance applies.", nil
	}
	return "Restored. House rules now:\n" + previous, nil
}
