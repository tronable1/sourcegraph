package bitbucketserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	eventTypeHeader = "X-Event-Key"
)

func WebHookType(r *http.Request) string {
	return r.Header.Get(eventTypeHeader)
}

func ParseWebHook(event string, payload []byte) (interface{}, error) {
	switch {
	case strings.HasPrefix(event, "pr:comment:"):
		var e PullRequestCommentEvent
		err := json.Unmarshal(payload, &e)
		if err != nil {
			return nil, err
		}
		e.Action = strings.TrimPrefix(event, "pr:comment:")
		return &e, nil
	case strings.HasPrefix(event, "pr:"):
		var e PullRequestEvent
		err := json.Unmarshal(payload, &e)
		if err != nil {
			return nil, err
		}
		e.Action = strings.TrimPrefix(event, "pr:")
		return &e, nil
	}

	return nil, errors.New("unsupported event")
}

func unmarshal(e interface{}, payload []byte) (interface{}, error) {
	return e, json.Unmarshal(payload, e)
}

type PullRequestEvent struct {
	Date        time.Time   `json:"data"`
	Actor       User        `json:"actor"`
	PullRequest PullRequest `json:"pullRequest"`
	Action      string
}

type PullRequestCommentEvent struct {
	PullRequestEvent
	Comment      Comment `json:"comment"`
	PreviousText string  `json:"previousText"`
	Action       string
}

// TODO: more events
