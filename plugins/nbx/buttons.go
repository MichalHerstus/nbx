package nbx

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
)

// runButtonOutput is the JSON payload returned by the button run endpoint.
type runButtonOutput struct {
	Action  string `json:"action"`
	Output  any    `json:"output,omitempty"`
	Message string `json:"message,omitempty"`
}

// runButton handles POST /api/nbx/buttons/{id}/run.
//
// The execution depends on the button "action":
//   - open_page: rejected (handled entirely in the UI, no server round-trip).
//   - run_js:    executes the button config "script" through the JSVM.
//   - webhook:   sends an HTTP request to the button "target" URL.
func runButton(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		button, err := app.FindRecordById(core.CollectionNameButtons, e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("The button does not exist.", err)
		}

		config, err := core.UnmarshalNbxButtonConfig(button.Get("config"))
		if err != nil {
			return e.BadRequestError("Invalid button config.", err)
		}

		action := strings.TrimSpace(button.GetString("action"))
		switch action {
		case core.ButtonActionOpenPage:
			return e.BadRequestError("This button is handled in the UI and cannot be run server-side.", nil)

		case core.ButtonActionRunJS:
			output, runErr := jsvm.RunScript(app, config.Script)
			if runErr != nil {
				return e.BadRequestError("Script execution failed: "+runErr.Error(), nil)
			}
			return e.JSON(http.StatusOK, runButtonOutput{Action: action, Output: output})

		case core.ButtonActionWebhook:
			status, body, webhookErr := runButtonWebhook(e.Request.Context(), strings.TrimSpace(button.GetString("target")), config)
			if webhookErr != nil {
				return e.BadRequestError("Webhook request failed: "+webhookErr.Error(), nil)
			}
			return e.JSON(http.StatusOK, runButtonOutput{
				Action:  action,
				Message: status,
				Output:  body,
			})

		default:
			return e.BadRequestError("Unknown button action.", nil)
		}
	}
}

// runButtonWebhook sends the configured HTTP request to the button target URL
// and returns the response status (e.g. "200 OK") and body.
func runButtonWebhook(ctx context.Context, target string, config *core.NbxButtonConfig) (string, string, error) {
	if target == "" {
		return "", "", nil //nolint:nilerr // empty target is a no-op
	}

	timeout := time.Duration(config.TimeoutSec) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if config.Body != "" {
		body = bytes.NewReader([]byte(config.Body))
	}

	req, err := http.NewRequestWithContext(reqCtx, config.Method, target, body)
	if err != nil {
		return "", "", err
	}
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	return resp.Status, string(respBody), nil
}
