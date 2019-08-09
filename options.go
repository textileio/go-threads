package threadstore

import (
	"github.com/ipfs/go-ipld-format"
)

// Opt is an instance helper for creating a thread option
var Opt Option

// Settings for a new thread
type Settings struct {
	Schema format.Node
	Roles  format.Node
	Intent string

	Name string
}

// Option returns a thread setting from an option
type Option func(*Settings)

// Schema sets the thread's immutable roles
func (Option) Schema(val format.Node) Option {
	return func(settings *Settings) {
		settings.Schema = val
	}
}

// Roles sets the thread's immutable roles
func (Option) Roles(val format.Node) Option {
	return func(settings *Settings) {
		settings.Roles = val
	}
}

// Intent sets the thread's immutable intention
func (Option) Intent(val string) Option {
	return func(settings *Settings) {
		settings.Intent = val
	}
}

// Name sets the thread's local name
func (Option) Name(val string) Option {
	return func(settings *Settings) {
		settings.Name = val
	}
}

// Options returns request settings from options
func Options(opts ...Option) *Settings {
	options := &Settings{
		// @todo: add default schema
		// @todo: add default roles
	}

	for _, opt := range opts {
		opt(options)
	}
	return options
}
