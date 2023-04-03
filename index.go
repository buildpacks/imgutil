package imgutil

type ImageIndex interface {
	Add(repoName string) error
	Remove(repoName string) error
	Save() error
}
