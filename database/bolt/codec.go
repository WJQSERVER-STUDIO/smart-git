package bolt

import (
	"encoding/gob"
	"io"
	"smart-git/database/schema"
)

func init() {
	gob.Register(&schema.RepoData{})
	gob.Register(&schema.RepoSumData{})
}

func encodeRepoData(w io.Writer, data *schema.RepoData) error {
	return gob.NewEncoder(w).Encode(data)
}

func decodeRepoData(r io.Reader, data *schema.RepoData) error {
	return gob.NewDecoder(r).Decode(data)
}

func encodeRepoSumData(w io.Writer, data *schema.RepoSumData) error {
	return gob.NewEncoder(w).Encode(data)
}

func decodeRepoSumData(r io.Reader, data *schema.RepoSumData) error {
	return gob.NewDecoder(r).Decode(data)
}
