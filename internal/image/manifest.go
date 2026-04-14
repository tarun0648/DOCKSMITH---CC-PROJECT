package image

// Manifest is the JSON representation of an image stored in ~/.docksmith/images/
type Manifest struct {
	Name    string  `json:"name"`
	Tag     string  `json:"tag"`
	Digest  string  `json:"digest"`
	Created string  `json:"created"`
	Config  Config  `json:"config"`
	Layers  []Layer `json:"layers"`
}

// Config holds runtime/build configuration for the image.
type Config struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
}

// Layer describes a single layer in the image.
type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	CreatedBy string `json:"createdBy"`
}
