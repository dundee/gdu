package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type selfUpdaterMock struct {
	res Result
	err error
}

func (m selfUpdaterMock) Update(context.Context) (Result, error) {
	return m.res, m.err
}

func TestSelfUpdateDisabledInDevelopmentBuild(t *testing.T) {
	buff := &bytes.Buffer{}

	outcome, err := Run(context.Background(), buff, New(), nil)

	assert.Equal(t, OutcomeExit, outcome)
	assert.ErrorContains(t, err, "cannot update itself")
}

func TestRunSelfUpdate(t *testing.T) {
	updated := Result{CurrentVersion: "v5.39.0", LatestVersion: "v5.40.0", Updated: true}
	upToDate := Result{CurrentVersion: "v5.40.0", LatestVersion: "v5.40.0"}
	failed := errors.New("network down")

	tests := []struct {
		name    string
		updater selfUpdaterMock
		args    []string
		want    Outcome
		wantErr string
		wantOut string
	}{
		{"updated, only flag", selfUpdaterMock{res: updated}, nil, OutcomeExit, "", "Updated gdu v5.39.0 -> v5.40.0"},
		{"updated, with args", selfUpdaterMock{res: updated}, []string{"/tmp"}, OutcomeRestart, "", "Updated gdu v5.39.0 -> v5.40.0"},
		{"up to date, only flag", selfUpdaterMock{res: upToDate}, nil, OutcomeExit, "", "gdu v5.40.0 is the latest version"},
		{"up to date, with args", selfUpdaterMock{res: upToDate}, []string{"-n"}, OutcomeContinue, "", "is the latest version"},
		{"failed, only flag", selfUpdaterMock{err: failed}, nil, OutcomeExit, "self-update: network down", ""},
		{"failed, with args", selfUpdaterMock{err: failed}, []string{"/tmp"}, OutcomeContinue, "", "Self-update failed: network down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buff := &bytes.Buffer{}

			outcome, err := Run(context.Background(), buff, tt.updater, tt.args)

			assert.Equal(t, tt.want, outcome)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Contains(t, buff.String(), tt.wantOut)
		})
	}
}

func TestStripSelfUpdateFlag(t *testing.T) {
	assert.Empty(t, StripFlag([]string{"--self-update"}))
	assert.Empty(t, StripFlag([]string{"--self-update=true"}))
	assert.Equal(t,
		[]string{"-n", "/tmp"},
		StripFlag([]string{"-n", "--self-update", "/tmp"}),
	)
	assert.Equal(t,
		[]string{"/tmp", "--", "--self-update"},
		StripFlag([]string{"--self-update", "/tmp", "--", "--self-update"}),
	)
}
