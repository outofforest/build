package tools

import "context"

type nameFiedType int

const nameField nameFiedType = iota

// WithName creates context with name embedded.
func WithName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameField, name)
}

// GetName returns name passed to `Main` function
func GetName(ctx context.Context) string {
	return ctx.Value(nameField).(string)
}
