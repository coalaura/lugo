package reporting

import (
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
)

const FlushTimeout = 2 * time.Second

// injected
var SentryDSN string

type Reporter struct {
	version string
	hub     *sentry.Hub
	enabled bool
}

func NewReporter(version string) (*Reporter, error) {
	if SentryDSN == "" {
		return nil, errors.New("missing DSN")
	}

	env := "release"

	if version == "dev" {
		env = "dev"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              SentryDSN,
		Environment:      env,
		Release:          "lugo@" + version,
		SendDefaultPII:   false,
		AttachStacktrace: true,
		DataCollection: &sentry.DataCollection{
			UserInfo: sentry.Set(false),
		},
	})

	if err != nil {
		return nil, err
	}

	return &Reporter{
		version: version,
		hub:     sentry.CurrentHub(),
		enabled: false,
	}, nil
}

func (r *Reporter) SetEnabled(enabled bool) {
	r.enabled = enabled
}

func (r *Reporter) Recover(err any, state map[string]any) {
	if !r.enabled || err == nil {
		return
	}

	event := eventFromPanic(err)

	trace := sentry.NewStacktrace()
	if trace != nil && len(event.Exception) > 0 {
		event.Exception[len(event.Exception)-1].Stacktrace = trace
	}

	if state != nil {
		event.Contexts["state"] = state
	}

	event.Contexts["app"] = sentry.Context{
		"app_name":    "lugo",
		"app_version": r.version,
	}

	r.hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelFatal)

		if state != nil {
			scope.SetContext("state", state)
		}

		r.hub.CaptureEvent(event)
	})

	r.hub.Flush(FlushTimeout)
}

func eventFromPanic(err any) *sentry.Event {
	event := sentry.NewEvent()
	event.Level = sentry.LevelFatal

	switch val := err.(type) {
	case error:
		event.SetException(val, -1)
	case string:
		event.Message = val
		event.Exception = []sentry.Exception{{
			Type:  "panic",
			Value: val,
		}}
	default:
		event.Message = fmt.Sprintf("%#v", val)
		event.Exception = []sentry.Exception{{
			Type:  "panic",
			Value: event.Message,
		}}
	}

	if len(event.Exception) > 0 {
		event.Exception[len(event.Exception)-1].Mechanism = &sentry.Mechanism{
			Type: "panic",
		}

		event.Exception[len(event.Exception)-1].Mechanism.SetUnhandled()
	}

	return event
}
