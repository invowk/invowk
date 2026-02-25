package checkall // want `stale exception: pattern "StalePattern.Field" matched no diagnostics`

// Mode has both IsValid and String — no supplementary diagnostics.
type Mode string

func (m Mode) IsValid() (bool, []error) { return m != "", nil }
func (m Mode) String() string            { return string(m) }

// MissingAll has neither IsValid nor String — flagged by both checks.
type MissingAll string // want `named type checkall\.MissingAll has no IsValid\(\) method` `named type checkall\.MissingAll has no String\(\) method`

// Server is an exported struct with no constructor — flagged.
type Server struct { // want `exported struct checkall\.Server has no NewServer\(\) constructor`
	Addr string // want `struct field checkall\.Server\.Addr uses primitive type string`
}

// Client has a constructor — not flagged.
type Client struct {
	host Mode
}

// NewClient is the constructor for Client.
func NewClient() *Client { return &Client{} }
