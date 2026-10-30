package api

import (
	"fmt"
)

// Comment 任务评论。
type Comment struct {
	ID          string `json:"id"`
	Actor       string `json:"actor"`
	CommentHTML string `json:"comment_html"`
	CreatedAt   string `json:"created_at"`
}

// Comments 列出任务的全部评论。
func (c *Client) Comments(projectID, issueID string) ([]Comment, error) {
	return Paginate[Comment](c,
		fmt.Sprintf("workspaces/%s/projects/%s/issues/%s/comments/", c.Workspace, projectID, issueID), 200)
}
