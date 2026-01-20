// VHS Output Normalization Configuration
//
// This file defines rules for normalizing VHS test output to produce
// deterministic, environment-independent golden files.
//
// See internal/vhsnorm/ for the normalizer implementation.

// VHS-specific artifact filtering
vhs_artifacts: {
	// Remove lines consisting only of box-drawing characters (frame separators)
	strip_frame_separators: true

	// Remove lines containing only the shell prompt character
	strip_empty_prompts: true

	// Remove consecutive duplicate lines (like `uniq`)
	deduplicate: true

	// The shell prompt character to detect empty prompts
	prompt_char: ">"

	// Box-drawing characters that form frame separators
	separator_chars: ["─", "━", "═", "│", "┃", "║"]
}

// Substitution rules applied in order
// Each rule replaces matches of `pattern` with `replacement`
substitutions: [
	// Timestamps
	{
		name:        "iso_timestamp"
		pattern:     "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}[A-Z]?"
		replacement: "[TIMESTAMP]"
	},

	// Home directories (Linux, macOS, Fedora Silverblue)
	{
		name:        "home_linux"
		pattern:     "/home/[a-zA-Z0-9_-]+"
		replacement: "[HOME]"
	},
	{
		name:        "home_var"
		pattern:     "/var/home/[a-zA-Z0-9_-]+"
		replacement: "[HOME]"
	},
	{
		name:        "home_macos"
		pattern:     "/Users/[a-zA-Z0-9_-]+"
		replacement: "[HOME]"
	},

	// Temporary directories
	{
		name:        "tmp_dir"
		pattern:     "/tmp/[a-zA-Z0-9._-]+"
		replacement: "[TMPDIR]"
	},
	{
		name:        "var_tmp_dir"
		pattern:     "/var/tmp/[a-zA-Z0-9._-]+"
		replacement: "[TMPDIR]"
	},

	// Hostnames
	{
		name:        "hostname"
		pattern:     "hostname: [a-zA-Z0-9._-]+"
		replacement: "hostname: [HOSTNAME]"
	},

	// Version strings
	{
		name:        "version_v_prefix"
		pattern:     "invowk v[0-9]+\\.[0-9]+\\.[0-9]+[^ ]*"
		replacement: "invowk [VERSION]"
	},
	{
		name:        "version_word"
		pattern:     "invowk version [0-9]+\\.[0-9]+\\.[0-9]+[^ ]*"
		replacement: "invowk version [VERSION]"
	},

	// Environment variable displays
	{
		name:        "env_user"
		pattern:     "USER = '[a-zA-Z0-9_-]+'"
		replacement: "USER = '[USER]'"
	},
	{
		name:        "env_home"
		pattern:     "HOME = '[^']+'"
		replacement: "HOME = '[HOME]'"
	},
	{
		name:        "env_path"
		pattern:     "PATH = '[^']+'"
		replacement: "PATH = '[PATH]'"
	},
	{
		name:        "env_path_truncated"
		pattern:     "PATH = '[^']+' \\(truncated\\)"
		replacement: "PATH = '[PATH]' (truncated)"
	},
]

// General content filters
filters: {
	// Remove ANSI escape sequences (colors, cursor movement, etc.)
	strip_ansi: true

	// Remove empty or whitespace-only lines
	strip_empty: true
}
