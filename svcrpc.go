package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	gitserver "github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/infinite-iroha/touka"
)

func serviceRPC(baseRepoDir string) touka.HandlerFunc {
	return func(c *touka.Context) {
		ctx := c.Request.Context()
		r := c.Request
		w := c.Writer
		svc := transport.UploadPackService

		repoName := c.Param("repo")
		if repoName == "" {
			renderStatusError(w, http.StatusBadRequest)
			return
		}
		userName := c.Param("user")

		endpoint := "/" + userName + "/" + repoName

		version := r.Header.Get("Git-Protocol")
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))

		expectedContentType := strings.ToLower(fmt.Sprintf("application/x-git-%s-request", svc.Name()))
		if contentType != expectedContentType {
			renderStatusError(w, http.StatusForbidden)
			c.Infof("Invalid Content-Type: %s, expected %s", contentType, expectedContentType)
			return
		}

		if err := ensureRepoReady(baseRepoDir, userName, repoName); err != nil {
			if err == plumbing.ErrReferenceNotFound {
				renderStatusError(w, http.StatusNotFound)
				return
			}

			logError("ensure repo failed: %v, repo: %s\n", err, repoName)
			renderStatusError(w, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", svc.Name()))
		w.Header().Set("Connection", "Keep-Alive")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		var reader io.ReadCloser
		var err error
		switch r.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(r.Body)
			if err != nil {
				logError("Error creating gzip reader: %v, repo: %s\n", err, repoName)
				renderStatusError(w, http.StatusBadRequest)
				return
			}
			defer reader.Close() //nolint:errcheck
		default:
			reader = r.Body
		}

		frw := &flushResponseWriter{ResponseWriter: w, log: nil, chunkSize: defaultChunkSize}

		ep, err := transport.NewEndpoint(endpoint)
		if err != nil {
			logError("Error creating endpoint: %v, repo: %s\n", err, repoName)
			renderStatusError(w, http.StatusBadRequest)
			return
		}

		bfs := osfs.New(baseRepoDir)
		ld := gitserver.NewFilesystemLoader(bfs, true)
		st, err := ld.Load(ep)
		if err != nil {
			logError("Error loading filesystem: %v, repo: %s\n", err, repoName)
			renderStatusError(w, http.StatusInternalServerError)
			return
		}

		switch svc {
		case transport.UploadPackService:
			err = transport.UploadPack(ctx, st, reader, frw,
				&transport.UploadPackOptions{
					GitProtocol:   version,
					AdvertiseRefs: false,
					StatelessRPC:  true,
				})
		case transport.ReceivePackService:
			err = transport.ReceivePack(ctx, st, reader, frw,
				&transport.ReceivePackOptions{
					GitProtocol:   version,
					AdvertiseRefs: false,
					StatelessRPC:  true,
				})
		default:
			renderStatusError(w, http.StatusBadRequest)
			return
		}
		if err != nil {
			renderStatusError(w, http.StatusInternalServerError)
			return
		}
	}
}
