package jsonplaceholder_test

import (
	"context"
	"testing"

	"github.com/postfinance/httpclient/example/jsonplaceholder"
)

func getClient(t *testing.T) *jsonplaceholder.Client {
	c, err := jsonplaceholder.NewClient(
		"https://jsonplaceholder.typicode.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCreate(t *testing.T) {
	c := getClient(t)
	p := jsonplaceholder.Post{
		UserID: 101,
		Title:  "Blog post title",
		Body:   "Blog post body",
	}
	post, _, err := c.Post.Create(context.Background(), &p)
	if err != nil {
		t.Error(err)
	}
	if post.ID == 0 {
		t.Error("post ID cannot be zero")
	}
	t.Logf("%#v\n", post)
}

func TestGet(t *testing.T) {
	c := getClient(t)
	post, _, err := c.Post.Get(context.Background(), 1)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%#v\n", post)
}

func TestList(t *testing.T) {
	c := getClient(t)
	posts, _, err := c.Post.List(context.Background())
	if err != nil {
		t.Error(err)
	}
	if len(posts) == 0 {
		t.Error("no posts found")
	}
	t.Logf("number of posts: %d\n", len(posts))
}

func TestDelete(t *testing.T) {
	c := getClient(t)
	_, err := c.Post.Delete(context.Background(), 1)
	if err != nil {
		t.Error(err)
	}
}
