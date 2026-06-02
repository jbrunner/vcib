package dispatcher

import "net/http"

// MatchesPatterns exposes the unexported matchesPatterns function for testing.
var MatchesPatterns = matchesPatterns

// NewTestDispatcher creates a Dispatcher with the given header patterns for testing.
func NewTestDispatcher(headerPatterns []string) *Dispatcher {
	return &Dispatcher{headerPatterns: headerPatterns}
}

// ForwardHeadersTest exposes the unexported forwardHeaders method for testing.
func (d *Dispatcher) ForwardHeadersTest(req *http.Request, src http.Header) {
	d.forwardHeaders(req, src)
}
