package p42

// Attachment represents an image or file attached to a task or turn prompt.
type Attachment struct {
	// Name is the display name of the attachment (e.g. "screenshot.png").
	Name string `json:"Name"`

	// MimeType is the MIME type (e.g. "image/png", "image/jpeg").
	MimeType string `json:"MimeType"`

	// S3Key is the S3 object key for uploaded attachments.
	// Mutually exclusive with RepoPath.
	S3Key string `json:"S3Key,omitempty"`

	// RepoPath is the repo-relative path for checked-in files (e.g. "docs/arch.png").
	// Mutually exclusive with S3Key.
	RepoPath string `json:"RepoPath,omitempty"`
}
