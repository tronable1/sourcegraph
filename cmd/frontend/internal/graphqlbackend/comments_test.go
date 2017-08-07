package graphqlbackend

import (
	"context"
	"reflect"
	"testing"

	sourcegraph "sourcegraph.com/sourcegraph/sourcegraph/pkg/api"
	store "sourcegraph.com/sourcegraph/sourcegraph/pkg/localstore"
)

func TestComments_appendUniqueEmailsFromMentions(t *testing.T) {
	tests := []struct {
		Input  string
		Output []string
	}{
		{Input: "+alice@gmail.com", Output: []string{"alice@gmail.com"}},
		{Input: "Hello there +alice@gmail.com", Output: []string{"alice@gmail.com"}},
		{Input: "+alice@gmail.com +alice@gmail.com", Output: []string{"alice@gmail.com"}},
		{Input: "Hello alice@gmail.com", Output: []string{}},
		{Input: "Hello alice@gmail.com", Output: []string{}},
		{Input: "Hello +alice@gmail.com and +bob@acme.com", Output: []string{"alice@gmail.com", "bob@acme.com"}},
	}

	for _, test := range tests {
		out := appendUniqueEmailsFromMentions(map[string]struct{}{}, []string{}, test.Input, "")
		if !reflect.DeepEqual(out, test.Output) {
			t.Errorf("expected %s for input \"%s\", got: %v", test.Output, test.Input, out)
		}
	}
}

func TestComments_emailsToNotify(t *testing.T) {
	one := &sourcegraph.Comment{
		AuthorEmail: "nick@sourcegraph.com",
		Contents:    "Yo +renfred@sourcegraph.com",
	}
	two := &sourcegraph.Comment{
		AuthorEmail: "nick@sourcegraph.com",
		Contents:    "Did you see this comment?",
	}
	three := &sourcegraph.Comment{
		AuthorEmail: "nick@sourcegraph.com",
		Contents:    "Going to mention myself to test notifications +nick@sourcegraph.com",
	}
	four := &sourcegraph.Comment{
		AuthorEmail: "renfred@sourcegraph.com",
		Contents:    "Dude, I am on vacation. Ask +sqs@sourcegraph.com or +john@sourcegraph.com",
	}
	five := &sourcegraph.Comment{
		AuthorEmail: "sqs@sourcegraph.com",
		Contents:    "Stop bothering Renfred!",
	}
	tests := []struct {
		previousComments []*sourcegraph.Comment
		newComment       *sourcegraph.Comment
		expected         []string
	}{
		{
			[]*sourcegraph.Comment{},
			one,
			[]string{"renfred@sourcegraph.com"},
		},
		{
			[]*sourcegraph.Comment{one},
			two,
			[]string{"renfred@sourcegraph.com"},
		},
		{
			[]*sourcegraph.Comment{one, two},
			three,
			[]string{"renfred@sourcegraph.com", "nick@sourcegraph.com"},
		},
		{
			[]*sourcegraph.Comment{one, two, three},
			four,
			[]string{"nick@sourcegraph.com", "sqs@sourcegraph.com", "john@sourcegraph.com"},
		},
		{
			[]*sourcegraph.Comment{one, two, three, four},
			five,
			[]string{"nick@sourcegraph.com", "renfred@sourcegraph.com", "john@sourcegraph.com"},
		},
	}
	for _, test := range tests {
		actual := emailsToNotify(test.previousComments, test.newComment)
		if !reflect.DeepEqual(actual, test.expected) {
			t.Fatalf("emailsToNotify(%+v, %+v) expected %#v; got %#v", test.previousComments, test.newComment, test.expected, actual)
		}
	}

}
func TestComments_Create(t *testing.T) {
	ctx := context.Background()

	repo := sourcegraph.LocalRepo{
		ID:          1,
		RemoteURI:   "github.com/foo/bar",
		AccessToken: "1234",
	}
	thread := sourcegraph.Thread{
		ID:             1,
		LocalRepoID:    1,
		File:           "foo.go",
		Revision:       "1234",
		StartLine:      1,
		EndLine:        2,
		StartCharacter: 3,
		EndCharacter:   4,
	}
	wantComment := sourcegraph.Comment{
		ThreadID:    1,
		Contents:    "Hello",
		AuthorName:  "Alice",
		AuthorEmail: "alice@acme.com",
	}

	store.Mocks.LocalRepos.MockGet_Return(t, &repo, nil)
	store.Mocks.Threads.MockGet_Return(t, &thread, nil)
	called, calledWith := store.Mocks.Comments.MockCreate(t)

	r := &schemaResolver{}
	_, err := r.AddCommentToThread(ctx, &struct {
		RemoteURI   string
		AccessToken string
		ThreadID    int32
		Contents    string
		AuthorName  string
		AuthorEmail string
	}{
		RemoteURI:   repo.RemoteURI,
		AccessToken: repo.AccessToken,
		ThreadID:    thread.ID,
		Contents:    wantComment.Contents,
		AuthorName:  wantComment.AuthorName,
		AuthorEmail: wantComment.AuthorEmail,
	})

	if err != nil {
		t.Error(err)
	}
	if !*called || !reflect.DeepEqual(wantComment, *calledWith) {
		t.Errorf("want Comments.Create call to be %v not %v", wantComment, *calledWith)
	}
}

func TestComments_CreateAccessDenied(t *testing.T) {
	ctx := context.Background()

	store.Mocks.LocalRepos.MockGet_Return(t, nil, store.ErrRepoNotFound)
	called, calledWith := store.Mocks.Comments.MockCreate(t)

	r := &schemaResolver{}
	comment, err := r.AddCommentToThread(ctx, &struct {
		RemoteURI   string
		AccessToken string
		ThreadID    int32
		Contents    string
		AuthorName  string
		AuthorEmail string
	}{})

	if *called {
		t.Errorf("should not have called Comments.Create (called with %v)", calledWith)
	}
	if comment != nil || err == nil {
		t.Error("did not return error for failed Comments.Create")
	}
}
