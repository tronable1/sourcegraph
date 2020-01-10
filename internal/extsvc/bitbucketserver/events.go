package bitbucketserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const (
	eventTypeHeader = "X-Event-Key"
)

func WebHookType(r *http.Request) string {
	return r.Header.Get(eventTypeHeader)
}

func ParseWebHook(event string, payload []byte) (interface{}, error) {
	switch event {
	case "pr:merged":
		var mergeEvent PullRequestMergeEvent
		return &mergeEvent, json.Unmarshal(payload, &mergeEvent)
	case "pr:declined":
		var declinedEvent PullRequestDeclinedEvent
		return &declinedEvent, json.Unmarshal(payload, &declinedEvent)
	case "pr:deleted":
		var deletedEvent PullRequestDeletedEvent
		return &deletedEvent, json.Unmarshal(payload, &deletedEvent)
	}

	return nil, errors.New("unsupported event")
}

type PullRequestEvent struct {
	Date        time.Time   `json:"data"`
	Actor       User        `json:"actor"`
	PullRequest PullRequest `json:"pullRequest"`
}

type PullRequestMergeEvent PullRequestEvent
type PullRequestDeclinedEvent PullRequestEvent
type PullRequestDeletedEvent PullRequestEvent
