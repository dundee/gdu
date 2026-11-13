package webui

import (
	"crypto/tls"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionURL(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:1234")
	assert.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:1234/?token=abc123", actionURL(addr, "abc123"))
}

func TestGenerateActionTokenIsRandomAndHexEncoded(t *testing.T) {
	a := generateActionToken()
	b := generateActionToken()

	assert.NotEqual(t, a, b, "two tokens from the same process must not collide")
	assert.Len(t, a, actionTokenSize*2, "hex-encoded token should be twice the byte length")
	assert.Regexp(t, "^[0-9a-f]+$", a)
}

func TestOriginOf(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	assert.Equal(t, "http://example.com", originOf(req))

	req.TLS = &tls.ConnectionState{}
	assert.Equal(t, "https://example.com", originOf(req))
}

func TestValidActionToken(t *testing.T) {
	ui := newTestUI()

	assert.True(t, ui.validActionToken(ui.actionToken))
	assert.False(t, ui.validActionToken(""))
	assert.False(t, ui.validActionToken("wrong"))
	assert.False(t, ui.validActionToken(ui.actionToken+"x"))
}

func TestSameOriginFetch(t *testing.T) {
	cases := []struct {
		name         string
		secFetchSite string
		origin       string
		host         string
		want         bool
	}{
		{name: "no headers allowed", want: true},
		{name: "same-origin fetch metadata allowed", secFetchSite: "same-origin", want: true},
		{name: "cross-site fetch metadata rejected", secFetchSite: "cross-site", want: false},
		{name: "same-site fetch metadata rejected", secFetchSite: "same-site", want: false},
		{name: "matching origin allowed", origin: "http://example.com", host: "example.com", want: true},
		{name: "mismatched origin rejected", origin: "http://attacker.example", host: "example.com", want: false},
		{
			name:         "fetch metadata takes priority over origin",
			secFetchSite: "cross-site",
			origin:       "http://example.com",
			host:         "example.com",
			want:         false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/reveal", nil)
			if c.host != "" {
				req.Host = c.host
			}
			if c.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", c.secFetchSite)
			}
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			assert.Equal(t, c.want, sameOriginFetch(req))
		})
	}
}
