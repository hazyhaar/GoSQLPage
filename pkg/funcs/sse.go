package funcs

import (
	"github.com/hazyhaar/gopage/pkg/sse"
	"zombiezen.com/go/sqlite"
)

// SSEFuncs returns SSE-related SQL functions.
func SSEFuncs() []Func {
	return []Func{
		{
			Name:          "sse_notify",
			NumArgs:       2, // channel, data
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				channel := args[0].Text()
				data := args[1].Text()

				hub := sse.GetHub()
				hub.Publish(channel, "message", data)

				return sqlite.TextValue("ok"), nil
			},
		},
		{
			Name:          "sse_notify_event",
			NumArgs:       3, // channel, event_type, data
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				channel := args[0].Text()
				eventType := args[1].Text()
				data := args[2].Text()

				hub := sse.GetHub()
				hub.Publish(channel, eventType, data)

				return sqlite.TextValue("ok"), nil
			},
		},
		{
			Name:          "sse_broadcast",
			NumArgs:       1, // data
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()

				hub := sse.GetHub()
				hub.Broadcast("message", data)

				return sqlite.TextValue("ok"), nil
			},
		},
		{
			Name:          "sse_client_count",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				hub := sse.GetHub()
				return sqlite.IntegerValue(int64(hub.ClientCount())), nil
			},
		},
		{
			Name:          "sse_channel_count",
			NumArgs:       1, // channel
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				channel := args[0].Text()
				hub := sse.GetHub()
				return sqlite.IntegerValue(int64(hub.ChannelCount(channel))), nil
			},
		},
	}
}
