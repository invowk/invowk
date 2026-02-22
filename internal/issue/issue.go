// SPDX-License-Identifier: MPL-2.0

package issue

import (
	"embed"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/charmbracelet/glamour"
)

// Issue IDs for different error scenarios.
const (
	FileNotFoundId Id = iota + 1
	InvowkfileNotFoundId
	InvowkfileParseErrorId
	CommandNotFoundId
	RuntimeNotAvailableId
	ContainerEngineNotFoundId
	DockerfileNotFoundId
	ScriptExecutionFailedId
	ConfigLoadFailedId
	InvalidRuntimeModeId
	ShellNotFoundId
	PermissionDeniedId
	DependenciesNotSatisfiedId
	HostNotSupportedId
	InvalidArgumentId
)

var (
	// ErrInvalidId is returned when an Id value is not one of the defined issue IDs.
	ErrInvalidId = errors.New("invalid issue id")
	// ErrInvalidMarkdownMsg is returned when a MarkdownMsg value is empty or whitespace-only.
	ErrInvalidMarkdownMsg = errors.New("invalid markdown message")
	// ErrInvalidHttpLink is returned when an HttpLink value does not have an http:// or https:// scheme.
	ErrInvalidHttpLink = errors.New("invalid http link")

	//go:embed templates
	templateFS embed.FS

	fileNotFoundIssue = &Issue{
		id:    FileNotFoundId,
		mdMsg: loadTemplate("file_not_found"),
	}

	invowkfileNotFoundIssue = &Issue{
		id:    InvowkfileNotFoundId,
		mdMsg: loadTemplate("invowkfile_not_found"),
	}

	invowkfileParseErrorIssue = &Issue{
		id:    InvowkfileParseErrorId,
		mdMsg: loadTemplate("invowkfile_parse_error"),
	}

	commandNotFoundIssue = &Issue{
		id:    CommandNotFoundId,
		mdMsg: loadTemplate("command_not_found"),
	}

	runtimeNotAvailableIssue = &Issue{
		id:    RuntimeNotAvailableId,
		mdMsg: loadTemplate("runtime_not_available"),
	}

	containerEngineNotFoundIssue = &Issue{
		id:    ContainerEngineNotFoundId,
		mdMsg: loadTemplate("container_engine_not_found"),
	}

	dockerfileNotFoundIssue = &Issue{
		id:    DockerfileNotFoundId,
		mdMsg: loadTemplate("dockerfile_not_found"),
	}

	scriptExecutionFailedIssue = &Issue{
		id:    ScriptExecutionFailedId,
		mdMsg: loadTemplate("script_execution_failed"),
	}

	configLoadFailedIssue = &Issue{
		id:    ConfigLoadFailedId,
		mdMsg: loadTemplate("config_load_failed"),
	}

	invalidRuntimeModeIssue = &Issue{
		id:    InvalidRuntimeModeId,
		mdMsg: loadTemplate("invalid_runtime_mode"),
	}

	shellNotFoundIssue = &Issue{
		id:    ShellNotFoundId,
		mdMsg: loadTemplate("shell_not_found"),
	}

	permissionDeniedIssue = &Issue{
		id:    PermissionDeniedId,
		mdMsg: loadTemplate("permission_denied"),
	}

	dependenciesNotSatisfiedIssue = &Issue{
		id:    DependenciesNotSatisfiedId,
		mdMsg: loadTemplate("dependencies_not_satisfied"),
	}

	hostNotSupportedIssue = &Issue{
		id:    HostNotSupportedId,
		mdMsg: loadTemplate("host_not_supported"),
	}

	invalidArgumentIssue = &Issue{
		id:    InvalidArgumentId,
		mdMsg: loadTemplate("invalid_argument"),
	}

	issues = map[Id]*Issue{
		fileNotFoundIssue.Id():             fileNotFoundIssue,
		invowkfileNotFoundIssue.Id():       invowkfileNotFoundIssue,
		invowkfileParseErrorIssue.Id():     invowkfileParseErrorIssue,
		commandNotFoundIssue.Id():          commandNotFoundIssue,
		runtimeNotAvailableIssue.Id():      runtimeNotAvailableIssue,
		containerEngineNotFoundIssue.Id():  containerEngineNotFoundIssue,
		dockerfileNotFoundIssue.Id():       dockerfileNotFoundIssue,
		scriptExecutionFailedIssue.Id():    scriptExecutionFailedIssue,
		configLoadFailedIssue.Id():         configLoadFailedIssue,
		invalidRuntimeModeIssue.Id():       invalidRuntimeModeIssue,
		shellNotFoundIssue.Id():            shellNotFoundIssue,
		permissionDeniedIssue.Id():         permissionDeniedIssue,
		dependenciesNotSatisfiedIssue.Id(): dependenciesNotSatisfiedIssue,
		hostNotSupportedIssue.Id():         hostNotSupportedIssue,
		invalidArgumentIssue.Id():          invalidArgumentIssue,
	}
)

type (
	// Id represents a unique identifier for an issue type.
	Id int

	// MarkdownMsg represents markdown-formatted issue message content.
	MarkdownMsg string

	// HttpLink represents a URL link for documentation or external resources.
	HttpLink string

	// InvalidIdError is returned when an Id value is not one of the defined issue IDs.
	// It wraps ErrInvalidId for errors.Is() compatibility.
	InvalidIdError struct {
		Value Id
	}

	// InvalidMarkdownMsgError is returned when a MarkdownMsg value is empty or whitespace-only.
	// It wraps ErrInvalidMarkdownMsg for errors.Is() compatibility.
	InvalidMarkdownMsgError struct {
		Value MarkdownMsg
	}

	// InvalidHttpLinkError is returned when an HttpLink value does not have a valid URL scheme.
	// It wraps ErrInvalidHttpLink for errors.Is() compatibility.
	InvalidHttpLinkError struct {
		Value HttpLink
	}

	// Issue represents a user-facing error with documentation and external links.
	Issue struct {
		id       Id          // ID used to lookup the issue
		mdMsg    MarkdownMsg // Markdown text that will be rendered
		docLinks []HttpLink  // optional links to invowk documentation
		extLinks []HttpLink  // external links that might be useful for the user
	}
)

// Error implements the error interface for InvalidIdError.
func (e *InvalidIdError) Error() string {
	return fmt.Sprintf("invalid issue id %d", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidIdError) Unwrap() error {
	return ErrInvalidId
}

// IsValid returns whether the Id is one of the defined issue IDs,
// and a list of validation errors if it is not.
func (id Id) IsValid() (bool, []error) {
	switch id {
	case FileNotFoundId, InvowkfileNotFoundId, InvowkfileParseErrorId,
		CommandNotFoundId, RuntimeNotAvailableId, ContainerEngineNotFoundId,
		DockerfileNotFoundId, ScriptExecutionFailedId, ConfigLoadFailedId,
		InvalidRuntimeModeId, ShellNotFoundId, PermissionDeniedId,
		DependenciesNotSatisfiedId, HostNotSupportedId, InvalidArgumentId:
		return true, nil
	default:
		return false, []error{&InvalidIdError{Value: id}}
	}
}

// Error implements the error interface for InvalidMarkdownMsgError.
func (e *InvalidMarkdownMsgError) Error() string {
	return "invalid markdown message: content is empty or whitespace-only"
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidMarkdownMsgError) Unwrap() error {
	return ErrInvalidMarkdownMsg
}

// IsValid returns whether the MarkdownMsg contains non-whitespace content,
// and a list of validation errors if it does not.
func (m MarkdownMsg) IsValid() (bool, []error) {
	if strings.TrimSpace(string(m)) == "" {
		return false, []error{&InvalidMarkdownMsgError{Value: m}}
	}
	return true, nil
}

// Error implements the error interface for InvalidHttpLinkError.
func (e *InvalidHttpLinkError) Error() string {
	return fmt.Sprintf("invalid http link %q (must start with http:// or https://)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidHttpLinkError) Unwrap() error {
	return ErrInvalidHttpLink
}

// IsValid returns whether the HttpLink has a valid URL scheme (http:// or https://),
// and a list of validation errors if it does not.
func (l HttpLink) IsValid() (bool, []error) {
	s := string(l)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false, []error{&InvalidHttpLinkError{Value: l}}
	}
	return true, nil
}

// Id returns the unique identifier for this issue.
func (i *Issue) Id() Id {
	return i.id
}

// MarkdownMsg returns the markdown-formatted message for this issue.
func (i *Issue) MarkdownMsg() MarkdownMsg {
	return i.mdMsg
}

// DocLinks returns a copy of the documentation links for this issue.
func (i *Issue) DocLinks() []HttpLink {
	return slices.Clone(i.docLinks)
}

// ExtLinks returns a copy of the external resource links for this issue.
func (i *Issue) ExtLinks() []HttpLink {
	return slices.Clone(i.extLinks)
}

// Render renders the issue message with documentation links using the specified style.
// It uses glamour.Render for markdown rendering.
func (i *Issue) Render(stylePath string) (string, error) {
	return i.RenderWith(glamour.Render, stylePath)
}

// RenderWith renders the issue message using the provided render function.
// This allows tests to inject a mock renderer without mutating package-level state.
func (i *Issue) RenderWith(renderFn func(string, string) (string, error), stylePath string) (string, error) {
	var extraMd strings.Builder
	if len(i.docLinks) > 0 || len(i.extLinks) > 0 {
		extraMd.WriteString("\n\n")
		extraMd.WriteString("## See also: ")
		for _, link := range i.docLinks {
			extraMd.WriteString("- [" + string(link) + "]")
		}
		for _, link := range i.extLinks {
			extraMd.WriteString("- [" + string(link) + "]")
		}
	}
	return renderFn(string(i.mdMsg)+extraMd.String(), stylePath)
}

// Values returns all registered issues.
func Values() []*Issue {
	return slices.Collect(maps.Values(issues))
}

// Get returns the issue with the given ID, or nil if not found.
func Get(id Id) *Issue {
	return issues[id]
}

// loadTemplate reads a markdown template from the embedded templates directory.
// It panics if the template is not found, which is intentional: templates are
// loaded at package init time, so any missing template is caught immediately
// by tests that import this package.
func loadTemplate(name string) MarkdownMsg {
	data, err := templateFS.ReadFile("templates/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("issue template %q not found: %v", name, err))
	}
	return MarkdownMsg(data)
}
