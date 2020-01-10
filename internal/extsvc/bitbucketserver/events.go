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
		return unmarshal(&mergeEvent, payload)
	case "pr:declined":
		var declinedEvent PullRequestDeclinedEvent
		return unmarshal(&declinedEvent, payload)
	case "pr:deleted":
		var deletedEvent PullRequestDeletedEvent
		return unmarshal(&deletedEvent, payload)
	case "pr:comment:added":
		var commentAddedEvent PullRequestCommentAddedEvent
		return unmarshal(&commentAddedEvent, payload)
	case "pr:comment:deleted":
		var commentDeletedEvent PullRequestCommentDeletedEvent
		return unmarshal(&commentDeletedEvent, payload)
	case "pr:comment:edited":
		var commentEditedEvent PullRequestCommentEditedEvent
		return unmarshal(&commentEditedEvent, payload)
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
}

type PullRequestMergeEvent PullRequestEvent
type PullRequestDeclinedEvent PullRequestEvent
type PullRequestDeletedEvent PullRequestEvent

type PullRequestCommentEvent struct {
	PullRequestEvent
	Comment Comment `json:"comment"`
}

type PullRequestCommentAddedEvent PullRequestCommentEvent
type PullRequestCommentDeletedEvent PullRequestCommentEvent

type PullRequestCommentEditedEvent struct {
	PullRequestCommentEvent
	PreviousText string `json:"previousText"`
}
