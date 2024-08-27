package tools

import "context"

type nameFiedType int

type versionFieldType int

const (
	nameField    nameFiedType     = iota
	versionField versionFieldType = iota
)

// WithName creates context with name embedded.
func WithName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameField, name)
}

// GetName returns name passed to `Main` function
func GetName(ctx context.Context) string {
	return ctx.Value(nameField).(string)
}

// WithVersion creates context with version embedded.
func WithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, versionField, version)
}

// GetVersion returns version passed to `Main` function
func GetVersion(ctx context.Context) string {
	return ctx.Value(versionField).(string)
}
