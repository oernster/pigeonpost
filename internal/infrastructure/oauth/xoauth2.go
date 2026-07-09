// Package oauth implements the Microsoft OAuth 2.0 authorization-code-with-PKCE flow over a loopback
// redirect, the XOAUTH2 SASL mechanism used to present the resulting bearer token to IMAP and SMTP, and
// the silent token refresh that keeps a signed-in account working. go-sasl ships no XOAUTH2 client, so
// the mechanism is implemented here.
package oauth

import "fmt"

// xoauth2Mech is the SASL mechanism name Microsoft (and Google) expect for bearer-token authentication.
const xoauth2Mech = "XOAUTH2"

// xoauth2Client is a sasl.Client for the XOAUTH2 mechanism. Its initial response carries the account
// address and the bearer access token; the mechanism is single-exchange, so a server challenge only ever
// arrives to report a failure, to which the client replies with an empty response before the server
// finishes the command with an error.
type xoauth2Client struct {
	username string
	token    string
}

// NewXOAUTH2Client builds an XOAUTH2 SASL client for the given account address and OAuth access token.
// It satisfies sasl.Client, so it can be passed to imapclient.Client.Authenticate and go-smtp
// Client.Auth.
func NewXOAUTH2Client(username, token string) *xoauth2Client {
	return &xoauth2Client{username: username, token: token}
}

// Start returns the XOAUTH2 mechanism name and the initial response bytes. The wire format is
// "user=<address>^Aauth=Bearer <token>^A^A", where ^A is a single 0x01 byte.
func (c *xoauth2Client) Start() (mech string, ir []byte, err error) {
	resp := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.token)
	return xoauth2Mech, []byte(resp), nil
}

// Next answers a server challenge. XOAUTH2 sends a challenge only to carry a base64 error payload when
// authentication fails; the client must reply with an empty (non-nil) response so the server can complete
// the exchange with its failure status.
func (c *xoauth2Client) Next(_ []byte) ([]byte, error) {
	return []byte{}, nil
}
