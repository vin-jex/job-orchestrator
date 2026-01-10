package observability

import "context"

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

func RequestIDKey() any {
	return requestIDKey
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}
