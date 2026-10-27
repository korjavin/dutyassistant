package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReportAssessment(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *ReportAssessment
		wantErr bool
	}{
		{name: "plain JSON", content: `{"score": 4, "analysis": "Good day."}`, want: &ReportAssessment{Score: 4, Analysis: "Good day."}},
		{name: "code fence", content: "```json\n{\"score\": 2, \"analysis\": \" Skipped chores. \"}\n```", want: &ReportAssessment{Score: 2, Analysis: "Skipped chores."}},
		{name: "group summary", content: `{"score": 5, "analysis": "Great.", "group_summary": " Helped everyone. "}`, want: &ReportAssessment{Score: 5, Analysis: "Great.", GroupSummary: "Helped everyone."}},
		{name: "no JSON", content: "I think 5", wantErr: true},
		{name: "score too high", content: `{"score": 6, "analysis": "x"}`, wantErr: true},
		{name: "score missing", content: `{"analysis": "x"}`, wantErr: true},
		{name: "broken JSON", content: `{"score": 4, "analysis": }`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReportAssessment(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAssessDailyReport(t *testing.T) {
	var gotReq chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"score\": 5, \"analysis\": \"Helped everyone.\", \"group_summary\": \"Helped at home.\"}"}}]}`))
	}))
	defer server.Close()

	c := NewClient("key", server.URL, 5, "test-model", nil)
	got, err := c.AssessDailyReport(context.Background(), "Masha", "Я помыла посуду", "- Was on duty today.\n", "")
	require.NoError(t, err)
	assert.Equal(t, &ReportAssessment{Score: 5, Analysis: "Helped everyone.", GroupSummary: "Helped at home."}, got)

	require.Len(t, gotReq.Messages, 2)
	assert.Equal(t, dailyReportSystemPrompt, gotReq.Messages[0].Content)
	assert.Contains(t, gotReq.Messages[0].Content, `"group_summary"`)
	assert.Contains(t, gotReq.Messages[1].Content, "Participant: Masha")
	assert.Contains(t, gotReq.Messages[1].Content, "- Was on duty today.")
	assert.Contains(t, gotReq.Messages[1].Content, "<report>\nЯ помыла посуду\n</report>")

	var nilClient *Client
	_, err = nilClient.AssessDailyReport(context.Background(), "Masha", "x", "", "")
	assert.Error(t, err)

	// The admin's house rules go into the system prompt, after the fixed part.
	_, err = c.AssessDailyReport(context.Background(), "Masha", "x", "", "  1. Не снижай за нечищеные зубы.\n")
	require.NoError(t, err)
	system := gotReq.Messages[0].Content
	assert.True(t, strings.HasPrefix(system, dailyReportSystemPrompt))
	assert.Contains(t, system, "<house_rules>\n1. Не снижай за нечищеные зубы.\n</house_rules>")
	assert.Equal(t, dailyReportSystemPrompt, dailyReportSystem(" \n "))
}

func TestTranscriber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/audio/transcriptions", r.URL.Path)
		assert.Equal(t, "Bearer stt-key", r.Header.Get("Authorization"))
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "whisper-1", r.FormValue("model"))
		assert.Equal(t, "ru", r.FormValue("language"))
		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		assert.Equal(t, "voice.ogg", header.Filename)
		data, _ := io.ReadAll(file)
		assert.Equal(t, "OGG-BYTES", string(data))
		_, _ = w.Write([]byte(`{"text": " Сегодня я помыл посуду. "}`))
	}))
	defer server.Close()

	tr := NewTranscriber("stt-key", server.URL+"/", 5, "", "ru")
	text, err := tr.Transcribe(context.Background(), strings.NewReader("OGG-BYTES"), "voice.ogg")
	require.NoError(t, err)
	assert.Equal(t, "Сегодня я помыл посуду.", text)
}

func TestTranscriber_Errors(t *testing.T) {
	assert.Nil(t, NewTranscriber("", "", 5, "", ""))

	var nilTr *Transcriber
	_, err := nilTr.Transcribe(context.Background(), strings.NewReader("x"), "voice.ogg")
	assert.Error(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"unsupported format"}`))
	}))
	defer server.Close()
	_, err = NewTranscriber("k", server.URL, 5, "", "").Transcribe(context.Background(), strings.NewReader("x"), "voice.ogg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestAssessDailyReport_MissingReport(t *testing.T) {
	var gotReq chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"score\": 3, \"analysis\": \"No report.\", \"group_summary\": \"No report today.\"}"}}]}`))
	}))
	defer server.Close()

	c := NewClient("key", server.URL, 5, "test-model", nil)
	got, err := c.AssessDailyReport(context.Background(), "Masha", " ", "- Completed no chores today.\n", "")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Score)
	require.Len(t, gotReq.Messages, 2)
	assert.Contains(t, gotReq.Messages[1].Content, "Report: none, the participant did not send a report today.")
	assert.NotContains(t, gotReq.Messages[1].Content, "<report>")
}
