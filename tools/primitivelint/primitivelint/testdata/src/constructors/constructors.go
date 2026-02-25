package constructors

// Config has a NewConfig constructor — no diagnostic for constructor check.
type Config struct {
	Name string // want `struct field constructors\.Config\.Name uses primitive type string`
}

func NewConfig(name string) *Config { return &Config{Name: name} } // want `parameter "name" of constructors\.NewConfig uses primitive type string`

// Server has no NewServer constructor — should be flagged.
type Server struct { // want `exported struct constructors\.Server has no NewServer\(\) constructor`
	Addr string // want `struct field constructors\.Server\.Addr uses primitive type string`
}

// Client has no NewClient constructor — should be flagged.
type Client struct { // want `exported struct constructors\.Client has no NewClient\(\) constructor`
	URL string // want `struct field constructors\.Client\.URL uses primitive type string`
}

// unexportedStruct is unexported — should NOT be checked by constructors.
type unexportedStruct struct {
	data string // want `struct field constructors\.unexportedStruct\.data uses primitive type string`
}

// Options has NewOptions below.
type Options struct {
	Timeout int // want `struct field constructors\.Options\.Timeout uses primitive type int`
}

func NewOptions() *Options { return &Options{} }

// NamedType is not a struct — should NOT be checked by --check-constructors.
type NamedType string
