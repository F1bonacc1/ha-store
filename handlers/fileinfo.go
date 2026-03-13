package handlers

// FileInfo represents detailed file metadata for ls -l style output.
type FileInfo struct {
	Name        string `json:"name"`
	Size        uint64 `json:"size"`
	ModTime     string `json:"mod_time"`
	Permissions string `json:"permissions"`
	Owner       string `json:"owner"`
	Group       string `json:"group"`
}
