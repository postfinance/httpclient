package jsonplaceholder

import (
	"context"
	"fmt"
	"net/http"

	"github.com/postfinance/httpclient"
)

// Post a post entry
type Post struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// PostService interface defines service methods
type PostService interface {
	Get(context.Context, int) (*Post, *http.Response, error)
	List(context.Context) ([]Post, *http.Response, error)
	Create(context.Context, *Post) (*Post, *http.Response, error)
	Delete(context.Context, int) (*http.Response, error)
}

// PostImpl implements the PostService interface
type PostImpl struct {
	client *httpclient.Client
}

var _ PostService = &PostImpl{}

// Get a post by ID
func (p *PostImpl) Get(ctx context.Context, id int) (*Post, *http.Response, error) {
	req, err := p.client.NewRequest(http.MethodGet, fmt.Sprintf("/posts/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	post := Post{}
	resp, err := p.client.Do(ctx, req, &post)
	if err != nil {
		return nil, nil, err
	}
	return &post, resp, nil
}

// List all posts
func (p *PostImpl) List(ctx context.Context) ([]Post, *http.Response, error) {
	req, err := p.client.NewRequest(http.MethodGet, "/posts", nil)
	if err != nil {
		return nil, nil, err
	}
	posts := []Post{}
	resp, err := p.client.Do(ctx, req, &posts)
	if err != nil {
		return nil, nil, err
	}
	return posts, resp, nil
}

// Create a new post
func (p *PostImpl) Create(ctx context.Context, newPost *Post) (*Post, *http.Response, error) {
	req, err := p.client.NewRequest(http.MethodPost, "/posts", newPost)
	if err != nil {
		return nil, nil, err
	}
	post := Post{}
	resp, err := p.client.Do(ctx, req, &post)
	if err != nil {
		return nil, nil, err
	}
	return &post, resp, nil
}

// Delete a post by ID
func (p *PostImpl) Delete(ctx context.Context, id int) (*http.Response, error) {
	req, err := p.client.NewRequest(http.MethodDelete, fmt.Sprintf("/posts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
