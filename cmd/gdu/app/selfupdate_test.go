package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/pkg/selfupdate"
)

type selfUpdaterMock struct {
	res selfupdate.Result
	err error
}

func (m selfUpdaterMock) Update(context.Context) (selfupdate.Result, error) {
	return m.res, m.err
}

func TestSelfUpdateDisabledInDevelopmentBuild(t *testing.T) {
	buff := &bytes.Buffer{}

	outcome, err := RunSelfUpdate(context.Background(), buff, selfupdate.New(), nil)

	assert.Equal(t, SelfUpdateExit, outcome)
	assert.ErrorContains(t, err, "cannot update itself")
}

func TestRunSelfUpdate(t *testing.T) {
	updated := selfupdate.Result{CurrentVersion: "v5.39.0", LatestVersion: "v5.40.0", Updated: true}
	upToDate := selfupdate.Result{CurrentVersion: "v5.40.0", LatestVersion: "v5.40.0"}
	failed := errors.New("network down")

	tests := []struct {
		name    string
		updater selfUpdaterMock
		args    []string
		want    SelfUpdateOutcome
		wantErr string
		wantOut string
	}{
		{"updated, only flag", selfUpdaterMock{res: updated}, nil, SelfUpdateExit, "", "Updated gdu v5.39.0 -> v5.40.0"},
		{"updated, with args", selfUpdaterMock{res: updated}, []string{"/tmp"}, SelfUpdateRestart, "", "Updated gdu v5.39.0 -> v5.40.0"},
		{"up to date, only flag", selfUpdaterMock{res: upToDate}, nil, SelfUpdateExit, "", "gdu v5.40.0 is the latest version"},
		{"up to date, with args", selfUpdaterMock{res: upToDate}, []string{"-n"}, SelfUpdateContinue, "", "is the latest version"},
		{"failed, only flag", selfUpdaterMock{err: failed}, nil, SelfUpdateExit, "self-update: network down", ""},
		{"failed, with args", selfUpdaterMock{err: failed}, []string{"/tmp"}, SelfUpdateContinue, "", "Self-update failed: network down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buff := &bytes.Buffer{}

			outcome, err := RunSelfUpdate(context.Background(), buff, tt.updater, tt.args)

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
	assert.Empty(t, StripSelfUpdateFlag([]string{"--self-update"}))
	assert.Empty(t, StripSelfUpdateFlag([]string{"--self-update=true"}))
	assert.Equal(t,
		[]string{"-n", "/tmp"},
		StripSelfUpdateFlag([]string{"-n", "--self-update", "/tmp"}),
	)
	assert.Equal(t,
		[]string{"/tmp", "--", "--self-update"},
		StripSelfUpdateFlag([]string{"--self-update", "/tmp", "--", "--self-update"}),
	)
}
