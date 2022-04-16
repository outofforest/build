package build

import (
	"context"
)

type nameFiedType int

const nameField nameFiedType = iota

func withName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameField, name)
}

// GetName returns name passed to `Do` function
func GetName(ctx context.Context) string {
	return ctx.Value(nameField).(string)
}
